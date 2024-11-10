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

// StorageMode represents the storage mode of the server.
type StorageMode int

const (
	SnapshotMode StorageMode = iota
	AOFMode
	HybridMode
)

const (
	defaultSnapshotInterval = 300              // 5 minutes
	defaultMaxSnapshots     = 5                // Retain 5 snapshots
	defaultAOFMaxSize       = 10 * 1024 * 1024 // 10 MB
)

// Server represents the key-value store server.
type Server struct {
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

// NewServer creates a new Server instance with default configurations.
func NewServer() *Server {
	return &Server{
		store:            make(map[string][]byte),
		shutdown:         make(chan struct{}),
		aofFiles:         []string{},
		storageMode:      SnapshotMode, // Default storage mode
		snapshotInterval: defaultSnapshotInterval,
		maxSnapshots:     defaultMaxSnapshots,
		aofMaxSize:       defaultAOFMaxSize,
	}
}

// Set sets the value for a given key.
func (s *Server) Set(ctx context.Context, req *pb.SetRequest) (*emptypb.Empty, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.store[req.Key] = req.Value

	if s.storageMode == AOFMode || s.storageMode == HybridMode {
		if err := s.writeToAOF(fmt.Sprintf("SET %s %s\n", req.Key, string(req.Value))); err != nil {
			return nil, err
		}
	}

	return &emptypb.Empty{}, nil
}

// Get retrieves the value for a given key.
func (s *Server) Get(ctx context.Context, req *pb.GetRequest) (*pb.GetResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	value, exists := s.store[req.Key]
	if !exists {
		return &pb.GetResponse{}, nil
	}
	return &pb.GetResponse{Value: value}, nil
}

// Del deletes a key-value pair.
func (s *Server) Del(ctx context.Context, req *pb.DelRequest) (*pb.DelResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, exists := s.store[req.Key]
	if exists {
		delete(s.store, req.Key)
		if s.storageMode == AOFMode || s.storageMode == HybridMode {
			if err := s.writeToAOF(fmt.Sprintf("DEL %s\n", req.Key)); err != nil {
				return nil, err
			}
		}
	}
	return &pb.DelResponse{Success: exists}, nil
}

// Keys returns all the keys in the store.
func (s *Server) Keys(ctx context.Context, req *pb.KeysRequest) (*pb.KeysResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	keys := make([]string, 0, len(s.store))
	for key := range s.store {
		keys = append(keys, key)
	}
	return &pb.KeysResponse{Keys: keys}, nil
}

// Config updates the server's configuration.
func (s *Server) Config(ctx context.Context, req *pb.ConfigRequest) (*pb.ConfigResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.storageMode = StorageMode(req.StorageMode)

	// Handle snapshot configurations
	if s.storageMode == SnapshotMode || s.storageMode == HybridMode {

		if req.SnapshotInterval > 0 {
			s.snapshotInterval = int(req.SnapshotInterval)
		} else {
			s.snapshotInterval = defaultSnapshotInterval
		}

		if req.MaxSnapshots > 0 {
			s.maxSnapshots = int(req.MaxSnapshots)
		} else {
			s.maxSnapshots = defaultMaxSnapshots
		}

		// Start or restart snapshot ticker
		if s.snapshotTicker != nil {
			s.snapshotTicker.Stop()
		}
		s.snapshotTicker = time.NewTicker(time.Duration(s.snapshotInterval) * time.Second)
		go s.snapshotWorker()

	} else {
		// Stop snapshotting if not in snapshot mode
		if s.snapshotTicker != nil {
			s.snapshotTicker.Stop()
			s.snapshotTicker = nil
		}
	}

	// Handle AOF configurations
	if s.storageMode == AOFMode || s.storageMode == HybridMode {

		if req.AofMaxSize > 0 {
			s.aofMaxSize = req.AofMaxSize
		} else {
			s.aofMaxSize = defaultAOFMaxSize
		}

		// Open AOF file if not already open
		if s.aofFile == nil {
			if err := s.createNewAOFFile(); err != nil {
				return nil, err
			}
		}
	} else {
		// Close AOF file if AOF is disabled
		if s.aofFile != nil {
			if err := s.aofFile.Close(); err != nil {
				log.Printf("Error closing AOF file: %v", err)
			}
			s.aofFile = nil
		}
	}

	return &pb.ConfigResponse{Success: true}, nil
}

func (s *Server) writeToAOF(command string) error {
	if s.aofFile != nil {
		if _, err := s.aofFile.WriteString(command); err != nil {
			return fmt.Errorf("error writing to AOF: %w", err)
		}
		if err := s.aofFile.Sync(); err != nil {
			return fmt.Errorf("error syncing AOF file: %w", err)
		}
		s.checkAOFSize()
	}
	return nil
}

func (s *Server) checkAOFSize() {
	fileInfo, err := s.aofFile.Stat()
	if err != nil {
		log.Printf("Error getting AOF file info: %v", err)
		return
	}
	if fileInfo.Size() >= s.aofMaxSize {
		if err := s.aofFile.Close(); err != nil {
			log.Printf("Error closing AOF file: %v", err)
		}
		if err := s.createNewAOFFile(); err != nil {
			log.Printf("Error creating new AOF file: %v", err)
		}
	}
}

func (s *Server) createNewAOFFile() error {
	timestamp := time.Now().Format("20060102T150405")
	filename := fmt.Sprintf("appendonly-%s.aof", timestamp)
	filePath := filepath.Join("aof", filename)

	if err := os.MkdirAll("aof", 0755); err != nil {
		return fmt.Errorf("error creating AOF directory: %w", err)
	}

	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("error opening AOF file: %w", err)
	}
	s.aofFile = file
	s.aofFiles = append(s.aofFiles, filePath)
	log.Printf("Created new AOF file %s", filePath)
	return nil
}

func (s *Server) snapshotWorker() {
	for {
		select {
		case <-s.snapshotTicker.C:
			if err := s.takeSnapshot(); err != nil {
				log.Printf("Error taking snapshot: %v", err)
			}
		case <-s.shutdown:
			return
		}
	}
}

func (s *Server) takeSnapshot() error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	timestamp := time.Now().Format("20060102T150405")
	filename := fmt.Sprintf("snapshot-%s.rdb", timestamp)
	filePath := filepath.Join("snapshots", filename)

	if err := os.MkdirAll("snapshots", 0755); err != nil {
		return fmt.Errorf("error creating snapshots directory: %w", err)
	}

	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("error creating snapshot file: %w", err)
	}
	defer file.Close()

	encoder := gob.NewEncoder(file)
	if err := encoder.Encode(s.store); err != nil {
		return fmt.Errorf("error encoding snapshot: %w", err)
	}

	log.Printf("Snapshot saved to %s", filePath)
	s.cleanupOldSnapshots()
	return nil
}

func (s *Server) cleanupOldSnapshots() {
	entries, err := os.ReadDir("snapshots")
	if err != nil {
		log.Printf("Error reading snapshots directory: %v", err)
		return
	}

	// Convert entries to FileInfo
	fileInfos := make([]os.FileInfo, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue // Skip directories
		}
		info, err := entry.Info()
		if err != nil {
			log.Printf("Error getting file info for %s: %v", entry.Name(), err)
			continue
		}
		fileInfos = append(fileInfos, info)
	}

	// If the number of snapshots is within the limit, no cleanup is needed
	if len(fileInfos) <= s.maxSnapshots {
		return
	}

	// Sort the files by modification time (oldest first)
	sort.Slice(fileInfos, func(i, j int) bool {
		return fileInfos[i].ModTime().Before(fileInfos[j].ModTime())
	})

	// Delete the oldest snapshots
	numToRemove := len(fileInfos) - s.maxSnapshots
	for i := 0; i < numToRemove; i++ {
		filePath := filepath.Join("snapshots", fileInfos[i].Name())
		if err := os.Remove(filePath); err != nil {
			log.Printf("Error deleting old snapshot %s: %v", fileInfos[i].Name(), err)
		} else {
			log.Printf("Deleted old snapshot %s", fileInfos[i].Name())
		}
	}
}

func (s *Server) loadSnapshot() {
	entries, err := os.ReadDir("snapshots")
	if err != nil {
		log.Printf("Error reading snapshots directory: %v", err)
		return
	}

	// Convert entries to FileInfo
	fileInfos := make([]os.FileInfo, 0, len(entries))
	for _, entry := range entries {

		if entry.IsDir() {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			log.Printf("Error getting file info for %s: %v", entry.Name(), err)
			continue
		}
		fileInfos = append(fileInfos, info)
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
	if err := decoder.Decode(&s.store); err != nil {
		log.Printf("Error decoding snapshot: %v", err)
	} else {
		log.Printf("Snapshot %s loaded successfully.", latestSnapshot.Name())
	}
}

func (s *Server) replayAOF() {
	entries, err := os.ReadDir("aof")
	if err != nil {
		log.Printf("Error reading AOF directory: %v", err)
		return
	}

	// Convert entries to FileInfo
	fileInfos := make([]os.FileInfo, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue // Skip directories
		}
		info, err := entry.Info()
		if err != nil {
			log.Printf("Error getting file info for %s: %v", entry.Name(), err)
			continue
		}
		fileInfos = append(fileInfos, info)
	}

	// Sort files by modification time (oldest first)
	sort.Slice(fileInfos, func(i, j int) bool {
		return fileInfos[i].ModTime().Before(fileInfos[j].ModTime())
	})

	for _, fileInfo := range fileInfos {
		filePath := filepath.Join("aof", fileInfo.Name())
		s.replayAOFFile(filePath)
	}
}

func (s *Server) replayAOFFile(filePath string) {
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

func (s *Server) loadData() {
	s.loadSnapshot()
	if s.storageMode == AOFMode || s.storageMode == HybridMode {
		s.replayAOF()
	}
}

// Close gracefully shuts down the server.
func (s *Server) Close() {
	close(s.shutdown)
	if s.snapshotTicker != nil {
		s.snapshotTicker.Stop()
	}
	if s.aofFile != nil {
		if err := s.aofFile.Close(); err != nil {
			log.Printf("Error closing AOF file: %v", err)
		}
	}
}

func main() {
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	grpcServer := grpc.NewServer()
	kvServer := NewServer()

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
