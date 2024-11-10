package main

import (
	"bufio"
	"context"
	"encoding/gob"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	pb "github.com/jry0/personal-kv-store/kvstore"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
)

type StorageMode int

const (
	SnapshotMode StorageMode = iota
	AOFMode
	HybridMode
)

type server struct {
	pb.UnimplementedKeyValueStoreServer
	store            map[string][]byte
	mu               sync.RWMutex
	storageMode      StorageMode
	snapshotInterval int
	maxSnapshots     int
	aofMaxSize       int64
	aofFiles         []string
	aofFile          *os.File
	snapshotTicker   *time.Ticker
	shutdown         chan struct{}
}

func newServer() *server {
	return &server{
		store:        make(map[string][]byte),
		shutdown:     make(chan struct{}),
		aofFiles:     []string{},
		storageMode:  SnapshotMode,     // Default storage mode
		maxSnapshots: 5,                // Default number of snapshots to retain
		aofMaxSize:   10 * 1024 * 1024, // Default AOF max size: 10 MB
	}
}

func (s *server) Set(ctx context.Context, req *pb.SetRequest) (*emptypb.Empty, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.store[req.Key] = req.Value

	if s.storageMode == AOFMode || s.storageMode == HybridMode {
		s.writeToAOF(fmt.Sprintf("SET %s %s\n", req.Key, string(req.Value)))
	}

	return &emptypb.Empty{}, nil
}

func (s *server) Get(ctx context.Context, req *pb.GetRequest) (*pb.GetResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	value, exists := s.store[req.Key]
	if !exists {
		return &pb.GetResponse{}, nil
	}
	return &pb.GetResponse{Value: value}, nil
}

func (s *server) Del(ctx context.Context, req *pb.DelRequest) (*pb.DelResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, exists := s.store[req.Key]
	if exists {
		delete(s.store, req.Key)
		if s.storageMode == AOFMode || s.storageMode == HybridMode {
			s.writeToAOF(fmt.Sprintf("DEL %s\n", req.Key))
		}
	}
	return &pb.DelResponse{Success: exists}, nil
}

func (s *server) Keys(ctx context.Context, req *pb.KeysRequest) (*pb.KeysResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	keys := make([]string, 0, len(s.store))
	for key := range s.store {
		keys = append(keys, key)
	}
	return &pb.KeysResponse{Keys: keys}, nil
}

func (s *server) Config(ctx context.Context, req *pb.ConfigRequest) (*pb.ConfigResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.storageMode = StorageMode(req.StorageMode)

	if s.storageMode == SnapshotMode || s.storageMode == HybridMode {

		if req.SnapshotInterval > 0 {
			s.snapshotInterval = int(req.SnapshotInterval)
		} else {
			s.snapshotInterval = 300 // Default to 5 minutes
		}

		if req.MaxSnapshots > 0 {
			s.maxSnapshots = int(req.MaxSnapshots)
		} else {
			s.maxSnapshots = 5 // Default to 5 snapshots
		}

		if s.snapshotTicker != nil {
			s.snapshotTicker.Stop()
		}
		s.snapshotTicker = time.NewTicker(time.Duration(s.snapshotInterval) * time.Second)

		go s.snapshotWorker()
	} else {
		// Disable snapshotting
		if s.snapshotTicker != nil {
			s.snapshotTicker.Stop()
			s.snapshotTicker = nil
		}
	}

	// Handle AOF parameters if AOF is enabled
	if s.storageMode == AOFMode || s.storageMode == HybridMode {
		// Set AOF max size
		if req.AofMaxSize > 0 {
			s.aofMaxSize = req.AofMaxSize
		} else {
			s.aofMaxSize = 10 * 1024 * 1024 // Default to 10 MB
		}
		// Open AOF file if not already open
		if s.aofFile == nil {
			err := s.createNewAOFFile()
			if err != nil {
				return &pb.ConfigResponse{Success: false}, err
			}
		}
	} else {
		// Disable AOF
		if s.aofFile != nil {
			s.aofFile.Close()
			s.aofFile = nil
		}
	}

	return &pb.ConfigResponse{Success: true}, nil
}

func (s *server) writeToAOF(command string) {
	if s.aofFile != nil {
		_, err := s.aofFile.WriteString(command)
		if err != nil {
			log.Printf("Error writing to AOF: %v", err)
			return
		}
		s.aofFile.Sync() // Ensure data is flushed to disk

		s.checkAOFSize()
	}
}

func (s *server) checkAOFSize() {
	fileInfo, err := s.aofFile.Stat()
	if err != nil {
		log.Printf("Error getting AOF file info: %v", err)
		return
	}
	if fileInfo.Size() >= s.aofMaxSize {
		s.aofFile.Close()
		err := s.createNewAOFFile()
		if err != nil {
			log.Printf("Error creating new AOF file: %v", err)
		}
	}
}

func (s *server) createNewAOFFile() error {
	timestamp := time.Now().Format("20060102T150405")
	filename := fmt.Sprintf("appendonly-%s.aof", timestamp)
	filePath := filepath.Join("aof", filename)

	err := os.MkdirAll("aof", 0755)
	if err != nil {
		return fmt.Errorf("Error creating AOF directory: %v", err)
	}

	s.aofFile, err = os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("Error opening AOF file: %v", err)
	}
	s.aofFiles = append(s.aofFiles, filePath)
	log.Printf("Created new AOF file %s", filePath)
	return nil
}

func (s *server) snapshotWorker() {
	for {
		select {
		case <-s.snapshotTicker.C:
			s.takeSnapshot()
		case <-s.shutdown:
			return
		}
	}
}

func (s *server) takeSnapshot() {
	s.mu.RLock()
	defer s.mu.RUnlock()

	timestamp := time.Now().Format("20060102T150405")
	filename := fmt.Sprintf("snapshot-%s.rdb", timestamp)
	filePath := filepath.Join("snapshots", filename)

	err := os.MkdirAll("snapshots", 0755)
	if err != nil {
		log.Printf("Error creating snapshots directory: %v", err)
		return
	}

	file, err := os.Create(filePath)
	if err != nil {
		log.Printf("Error creating snapshot file: %v", err)
		return
	}
	defer file.Close()

	encoder := gob.NewEncoder(file)
	err = encoder.Encode(s.store)
	if err != nil {
		log.Printf("Error encoding snapshot: %v", err)
	} else {
		log.Printf("Snapshot saved to %s", filePath)
		s.cleanupOldSnapshots()
	}
}

// Helper function to convert []os.DirEntry to []os.FileInfo
func getFileInfos(entries []os.DirEntry) ([]os.FileInfo, error) {
	fileInfos := make([]os.FileInfo, 0, len(entries))
	for _, entry := range entries {
		// Skip directories if necessary
		if entry.IsDir() {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			return nil, err
		}
		fileInfos = append(fileInfos, info)
	}
	return fileInfos, nil
}

// Updated cleanupOldSnapshots function
func (s *server) cleanupOldSnapshots() {
	// Read directory entries using os.ReadDir
	entries, err := os.ReadDir("snapshots")
	if err != nil {
		log.Printf("Error reading snapshots directory: %v", err)
		return
	}

	// Convert entries to FileInfo using the helper function
	fileInfos, err := getFileInfos(entries)
	if err != nil {
		log.Printf("Error converting entries to FileInfo: %v", err)
		return
	}

	// If the number of snapshots is within the limit, no cleanup is needed
	if len(fileInfos) <= s.maxSnapshots {
		return
	}

	// Sort the files by modification time (oldest first)
	sort.Slice(fileInfos, func(i, j int) bool {
		return fileInfos[i].ModTime().Before(fileInfos[j].ModTime())
	})

	// Determine how many files to remove
	numToRemove := len(fileInfos) - s.maxSnapshots

	// Delete the oldest snapshots
	for i := 0; i < numToRemove; i++ {
		filePath := filepath.Join("snapshots", fileInfos[i].Name())
		err := os.Remove(filePath)
		if err != nil {
			log.Printf("Error deleting old snapshot %s: %v", fileInfos[i].Name(), err)
		} else {
			log.Printf("Deleted old snapshot %s", fileInfos[i].Name())
		}
	}
}

func (s *server) loadSnapshot() {
	entries, err := os.ReadDir("snapshots")
	if err != nil {
		log.Printf("Error reading snapshots directory: %v", err)
		return
	}

	// Convert entries to FileInfo using the helper function
	fileInfos, err := getFileInfos(entries)
	if err != nil {
		log.Printf("Error converting entries to FileInfo: %v", err)
		return
	}

	if len(fileInfos) == 0 {
		return
	}

	// Find the most recent snapshot
	sort.Slice(fileInfos, func(i, j int) bool {
		return fileInfos[i].ModTime().After(fileInfos[j].ModTime())
	})
	latestSnapshot := fileInfos[0]

	filePath := filepath.Join("snapshots", latestSnapshot.Name())
	file, err := os.Open(filePath)
	if err != nil {
		log.Printf("Error opening snapshot file: %v", err)
		return
	}
	defer file.Close()

	decoder := gob.NewDecoder(file)
	err = decoder.Decode(&s.store)
	if err != nil {
		log.Printf("Error decoding snapshot: %v", err)
	} else {
		log.Printf("Snapshot %s loaded successfully.", latestSnapshot.Name())
	}
}

func (s *server) replayAOF() {
	entries, err := os.ReadDir("snapshots")
	if err != nil {
		log.Printf("Error reading AOF directory: %v", err)
		return
	}

	// Convert entries to FileInfo using the helper function
	fileInfos, err := getFileInfos(entries)
	if err != nil {
		log.Printf("Error converting entries to FileInfo: %v", err)
		return
	}

	// Sort files by modification time (oldest first)
	sort.Slice(fileInfos, func(i, j int) bool {
		return fileInfos[i].ModTime().Before(fileInfos[j].ModTime())
	})

	for _, file := range fileInfos {
		filePath := filepath.Join("aof", file.Name())
		s.replayAOFFile(filePath)
	}
}

func (s *server) replayAOFFile(filePath string) {
	file, err := os.Open(filePath)
	if err != nil {
		log.Printf("Error opening AOF file %s: %v", filePath, err)
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Fields(line)
		if len(parts) == 0 {
			continue
		}
		cmd := parts[0]
		switch cmd {
		case "SET":
			if len(parts) >= 3 {
				key := parts[1]
				value := []byte(strings.Join(parts[2:], " "))
				s.store[key] = value
			}
		case "DEL":
			if len(parts) == 2 {
				key := parts[1]
				delete(s.store, key)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		log.Printf("Error reading AOF file %s: %v", filePath, err)
	} else {
		log.Printf("Replayed AOF file %s", filePath)
	}
}

func (s *server) loadData() {
	s.loadSnapshot()
	if s.storageMode == AOFMode || s.storageMode == HybridMode {
		s.replayAOF()
	}
}

func (s *server) Close() {
	close(s.shutdown)
	if s.snapshotTicker != nil {
		s.snapshotTicker.Stop()
	}
	if s.aofFile != nil {
		s.aofFile.Close()
	}
}

func main() {
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	grpcServer := grpc.NewServer()
	kvServer := newServer()

	// Load data from snapshots and AOF
	kvServer.loadData()

	pb.RegisterKeyValueStoreServer(grpcServer, kvServer)

	// Handle graceful shutdown
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
		<-sigChan
		log.Println("Shutting down server...")
		kvServer.Close()
		grpcServer.GracefulStop()
		os.Exit(0)
	}()

	log.Println("Server is listening on port 50051")
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}
