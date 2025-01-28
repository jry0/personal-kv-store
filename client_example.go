package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	pb "github.com/jry0/personal-kv-store/kvstore"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {

	conn, err := grpc.NewClient("kvstore_server:50051", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}

	defer conn.Close()
	client := pb.NewKeyValueStoreClient(conn)

	reader := bufio.NewReader(os.Stdin)
	fmt.Println("Interactive Key-Value Store Client")
	fmt.Println("Available commands: set, get, del, keys, config, exit")

	for {
		fmt.Print("> ")
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)

		if input == "" {
			continue
		}

		parts := strings.Fields(input)
		cmd := strings.ToLower(parts[0])

		switch cmd {
		case "set":
			if len(parts) < 3 {
				fmt.Println("Usage: set <key> <value>")
				continue
			}
			key := parts[1]
			value := strings.Join(parts[2:], " ")
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			_, err := client.Set(ctx, &pb.SetRequest{Key: key, Value: []byte(value)})
			if err != nil {
				fmt.Printf("Set failed: %v\n", err)
			} else {
				fmt.Println("Set operation successful")
			}
		case "get":
			if len(parts) != 2 {
				fmt.Println("Usage: get <key>")
				continue
			}
			key := parts[1]
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			res, err := client.Get(ctx, &pb.GetRequest{Key: key})
			if err != nil {
				fmt.Printf("Get failed: %v\n", err)
			} else if len(res.Value) == 0 {
				fmt.Println("Key not found")
			} else {
				fmt.Printf("Value: %s\n", string(res.Value))
			}
		case "del":
			if len(parts) != 2 {
				fmt.Println("Usage: del <key>")
				continue
			}
			key := parts[1]
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			res, err := client.Del(ctx, &pb.DelRequest{Key: key})
			if err != nil {
				fmt.Printf("Del failed: %v\n", err)
			} else if res.Success {
				fmt.Println("Delete operation successful")
			} else {
				fmt.Println("Key not found")
			}
		case "keys":
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			res, err := client.Keys(ctx, &pb.KeysRequest{})
			if err != nil {
				fmt.Printf("Keys failed: %v\n", err)
			} else {
				fmt.Printf("Keys: %v\n", res.Keys)
			}
		case "config":
			if len(parts) < 2 {
				fmt.Println("Usage: config <storage_mode> [snapshot_interval] [max_snapshots] [aof_max_size]")
				fmt.Println("Storage modes: NON_PERSISTENT, SNAPSHOT, AOF, HYBRID")
				continue
			}

			storageModeStr := strings.ToUpper(parts[1])
			var storageMode pb.StorageMode
			switch storageModeStr {
			case "NON_PERSISTENT":
				storageMode = pb.StorageMode_NON_PERSISTENT
			case "SNAPSHOT":
				storageMode = pb.StorageMode_SNAPSHOT
			case "AOF":
				storageMode = pb.StorageMode_AOF
			case "HYBRID":
				storageMode = pb.StorageMode_HYBRID
			default:
				fmt.Println("Invalid storage mode. Choose from NON_PERSISTENT, SNAPSHOT, AOF, HYBRID")
				continue
			}

			var snapshotInterval int32
			var maxSnapshots int32
			var aofMaxSize int64

			// Determine required parameters based on storage mode
			if storageMode == pb.StorageMode_SNAPSHOT || storageMode == pb.StorageMode_HYBRID {
				if len(parts) < 4 {
					fmt.Println("Usage for SNAPSHOT or HYBRID mode: config <storage_mode> <snapshot_interval> <max_snapshots> [aof_max_size]")
					continue
				}
				si, err := strconv.ParseInt(parts[2], 10, 32)
				if err != nil || si <= 0 {
					fmt.Println("Invalid snapshot_interval. It must be a positive integer representing seconds.")
					continue
				}
				snapshotInterval = int32(si)

				ms, err := strconv.ParseInt(parts[3], 10, 32)
				if err != nil || ms <= 0 {
					fmt.Println("Invalid max_snapshots. It must be a positive integer.")
					continue
				}
				maxSnapshots = int32(ms)
			}

			if storageMode == pb.StorageMode_AOF || storageMode == pb.StorageMode_HYBRID {
				if storageMode == pb.StorageMode_HYBRID && len(parts) < 5 {
					fmt.Println("Usage for HYBRID mode: config HYBRID <snapshot_interval> <max_snapshots> <aof_max_size>")
					continue
				} else if storageMode == pb.StorageMode_AOF && len(parts) < 3 {
					fmt.Println("Usage for AOF mode: config AOF <aof_max_size>")
					continue
				}

				aofSizeIndex := 4
				if storageMode == pb.StorageMode_AOF {
					aofSizeIndex = 2
				}

				aof, err := strconv.ParseInt(parts[aofSizeIndex], 10, 64)
				if err != nil || aof <= 0 {
					fmt.Println("Invalid aof_max_size. It must be a positive integer representing bytes.")
					continue
				}
				aofMaxSize = aof
			}

			// Construct the ConfigRequest based on storage mode
			configReq := &pb.ConfigRequest{
				StorageMode: storageMode,
			}

			if storageMode == pb.StorageMode_SNAPSHOT || storageMode == pb.StorageMode_HYBRID {
				configReq.SnapshotInterval = snapshotInterval
				configReq.MaxSnapshots = maxSnapshots
			}

			if storageMode == pb.StorageMode_AOF || storageMode == pb.StorageMode_HYBRID {
				configReq.AofMaxSize = aofMaxSize
			}

			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			res, err := client.Config(ctx, configReq)
			if err != nil {
				fmt.Printf("Config failed: %v\n", err)
			} else if res.Success {
				fmt.Println("Config operation successful")
			} else {
				fmt.Println("Config operation failed")
			}
		case "exit":
			fmt.Println("Exiting client.")
			return
		default:
			fmt.Println("Unknown command. Available commands: set, get, del, keys, config, exit")
		}
	}
}
