# Personal Key-Value Store

## Overview

This project implements a personal key-value store using Go and gRPC. It supports various storage modes, including:

- **Non-Persistent Mode**: Data is stored only in memory (non-persistent).
- **Snapshot Mode**: Data is periodically saved to snapshot files.
- **Append-Only File (AOF) Mode**: All write operations are logged to disk.
- **Hybrid Mode**: Combines both Snapshot and AOF methods for persistence.

The server provides a gRPC interface for clients to interact with the key-value store.

The project includes:

- A gRPC server (`server.go`).
- An interactive client (`client_example.go`) that allows you to interact with the server using commands.
- Separate Dockerfiles for server and client.
- Instructions on how to build, run, and test the server and client using Docker.
- Steps to configure the server from the interactive client.

### Future Development ###
- Implement distributed architecture with:
  - consistent hashing
  - virtual node replication
  - RAFT leader election

## Table of Contents

- [Prerequisites](#prerequisites)
- [Project Structure](#project-structure)
- [Building the Server and Client Separately](#building-the-server-and-client-separately)
  - [Building the Server Docker Image](#building-the-server-docker-image)
  - [Building the Client Docker Image](#building-the-client-docker-image)
- [Running the Docker Containers](#running-the-docker-containers)
  - [Running the Server Container](#running-the-server-container)
  - [Running the Client Container](#running-the-client-container)
- [Using the Interactive Client](#using-the-interactive-client)
- [Configuring the Server from the Client](#configuring-the-server-from-the-client)
- [Writing Your Own Client](#writing-your-own-client)
- [License](#license)
- [Style](#style)

## Prerequisites

- **Docker**: Ensure Docker is installed on your system. You can download it from the [official website](https://www.docker.com/get-started).
- **Git**: For cloning the repository.

## Project structure
- `kvstore.proto`: Protobuf definitions of the gRPC service.
- `server.go`: Implementation of the gRPC server.
- `client_example.go`: Interactive client to interact with the server.
- `Dockerfile.server`: Dockerfile to containerize the server.
- `Dockerfile.client`: Dockerfile to containerize the client.
- `kvstore/`: Contains the generated Go code from the `.proto` file.
- `go.mod` and `go.sum`: Go module files.
- `README.md`: This file.

## Building the Server and Client Separately

1. **Clone the Repository**

    ```bash
    git clone https://github.com/jry0/personal-kv-store.git
    cd personal-kv-store
    ```
   
2. **Build the Server Image**

    Use the `Dockerfile.server` to build the server Docker image.

    ```bash
    docker build -t kvstore_server:latest -f Dockerfile.server .
    ```

3. **Build the Client Image**

    Use the `Dockerfile.client` to build the client Docker image.
    
    ```bash
    docker build -t kvstore_client:latest -f Dockerfile.client .
    ```

## Running the Server Container

### Create a User-Defined Network
  This allows the client to communicate with the server using the container name.

  ```bash
  docker network create kvstore_net
  ```
    
### Run the Server Container on the Network
  ```bash
  docker run -d --name kvstore_server --network kvstore_net -p 50051:50051 kvstore_server:latest
  ```
  Explanation:

  `-d`: Runs the container in detached mode (in the background).
  
  `--name kvstore_server`: Names the container kvstore_server for easier reference.
  
  `--network kvstore_net`: Connects the container to the kvstore_net network.
  
  `-p 50051:50051`: Maps port 50051 of the host to port 50051 of the container.
  
  `kvstore_server:latest`: Specifies the image to run.
    
  **Note**: The server will automatically create necessary directories (snapshots and aof) inside the container as needed.

## Running the Client Container

### Run the Client Container on the Same Network

```bash
docker run -it --rm --name kvstore_client --network kvstore_net kvstore_client:latest
```
Explanation:

`-it`: Runs the container in interactive mode with a TTY.
`--rm`: Automatically removes the container when it exits.
`--name kvstore_client`: Names the container kvstore_client.
`--network kvstore_net`: Connects the container to the kvstore_net network.
`kvstore_client:latest`: Specifies the image to run.

### Alternative Option (Using Host Networking on Linux):

If you are on Linux and prefer using host networking, you can run the client with the `--network host` option. This allows the client to connect to localhost:50051.

```bash
docker run -it --rm --name kvstore_client --network host kvstore_client:latest
```
**Note**: The --network host option is not recommended for macOS and Windows as host networking behaves differently.


## Using the Interactive Client
Once the client container is running, you'll see the interactive prompt:

```bash 
Interactive Key-Value Store Client
Available commands: set, get, del, keys, config, exit
>
```
### Available Commands:

#### Set a Key-Value Pair
```bash
set <key> <value>
```

Example:
```bash
> set username johndoe
Set operation successful
```
#### Get the Value for a Key

```bash
get <key>
```

Example:

```bash
> get username
Value: johndoe
```

#### Delete a Key

```bash
del <key>
```
Example:

```bash
> del username
Delete operation successful
```

#### List All Keys

```
keys
```
Example:

```bash
> keys
Keys: [username]
```

### Configure the Server

```bash
config <storage_mode> <snapshot_interval> <max_snapshots> <aof_max_size>
``` 
#### Storage Modes:

`NON_PERSISTENT`: Data is stored only in memory (non-persistent).

`SNAPSHOT`: Data is periodically saved to snapshot files.

`AOF`: All write operations are logged to disk.

`HYBRID`: Combines both Snapshot and AOF methods for persistence.
Usage Based on Storage Mode:

**NON_PERSISTENT**:
```bash
config NON_PERSISTENT
```

**SNAPSHOT**:

```bash
config SNAPSHOT <snapshot_interval> <max_snapshots>
```
Example:

```bash
> config SNAPSHOT 300 5
Config operation successful
```


**AOF**:

```bash
config AOF <aof_max_size>
```
Example:

```bash
> config AOF 10485760
Config operation successful
```

**HYBRID**:

```bash
config HYBRID <snapshot_interval> <max_snapshots> <aof_max_size>
```
Example:

```bash
> config HYBRID 300 5 10485760
Config operation successful
```
#### Parameter Definitions:

`<storage_mode>:` NON_PERSISTENT, SNAPSHOT, AOF, or HYBRID.

`<snapshot_interval>`: Interval in seconds between snapshots (e.g., 300 for 5 minutes).

`<max_snapshots>`: Maximum number of snapshots to retain (e.g., 5).

`<aof_max_size>`: Maximum size of AOF files in bytes (e.g., 10485760 for 10 MB).

### Exit the Client

```bash
exit
```
Example:

```bash
> exit
Exiting client.
```


### Style ##
- Following https://google.github.io/styleguide/go/ for Go files.
- Following https://protobuf.dev/programming-guides/style/ for proto files.
- TODO: Implement linter / pre-commits