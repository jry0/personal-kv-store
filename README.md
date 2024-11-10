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
- `kvstore/`: Contains the generated Go code from the `.proto` file.

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

_Example:_
```bash
> set username johndoe
Set operation successful
```
#### Get the Value for a Key

```bash
get <key>
```

_Example:_

```bash
> get username
Value: johndoe
```

#### Delete a Key

```bash
del <key>
```
_Example:_

```bash
> del username
Delete operation successful
```

#### List All Keys

```
keys
```
_Example:_

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
_Example:_

```bash
> config SNAPSHOT 300 5
Config operation successful
```


**AOF**:

```bash
config AOF <aof_max_size>
```
_Example:_

```bash
> config AOF 10485760
Config operation successful
```

**HYBRID**:

```bash
config HYBRID <snapshot_interval> <max_snapshots> <aof_max_size>
```
_Example:_

```bash
> config HYBRID 300 5 10485760
Config operation successful
```
#### Config Parameter Definitions:

**Parameter Details**:

- `<storage_mode>`: Defines how data persistence is handled.

  - `NON_PERSISTENT`: Operates entirely in memory without saving data to disk.

  - `SNAPSHOT`: Periodically saves data to snapshot files.

  - `AOF`: Logs all write operations to an Append-Only File for recovery.

  - `HYBRID`: Combines both Snapshot and AOF methods for robust persistence.

- `<snapshot_interval>`: (Applicable for `SNAPSHOT` and `HYBRID` modes)

  - **Type**: Integer
  - **Description**: Interval in seconds between automatic snapshots.
  - **Example**: 300 (for 5 minutes)
- `<max_snapshots>`: (Applicable for `SNAPSHOT` and `HYBRID` modes)

  - **Type**: Integer
  - **Description**: Maximum number of snapshot files to retain.
  - **Example**: 5

- `<aof_max_size>`: (Applicable for `AOF` and `HYBRID` modes)

  - **Type**: Integer
  - **Description**: Maximum size of each AOF file in bytes.
  - **Example**: 10485760 (for 10 MB)


**Note**: When configuring the server, ensure that you provide the correct number of parameters based on the selected storage mode. The client will prompt and validate the parameters accordingly.
### Exit the Client

```bash
exit
```
_Example:_

```bash
> exit
Exiting client.
```

### Writing Your Own Client
To create your own client in Go or another language, follow these steps:

1. Define the Protobuf Messages and Service

2. Use the kvstore.proto file as a reference for the gRPC service definitions and message formats.

3. Generate Client Code

    *Use the appropriate Protobuf plugin for your language to generate the client code.*

    **For Go**:

    ```bash
    protoc \
      --go_out=./kvstore \
      --go-grpc_out=./kvstore \
      --go_opt=paths=source_relative \
      --go-grpc_opt=paths=source_relative \
      kvstore.proto
    ```
4. Establish a Connection to the Server

    *Connect to the server at kvstore_server:50051 (when using Docker network) or localhost:50051 (if running client locally).*

    *See [`client_example.go`](https://github.com/jry0/personal-kv-store/blob/main/client_example.go) as reference*

5. Use the client stub to call the Set, Get, Del, Keys, and Config methods.

    *See [`client_example.go`](https://github.com/jry0/personal-kv-store/blob/main/client_example.go) as reference*
    
**For Other Languages:**

Generate client code using the respective Protobuf plugin (e.g., grpc_tools_node_protoc for Node.js, grpcio-tools for Python).
Follow the language-specific conventions for creating stubs and making RPC calls.
### Style ##
- Following https://google.github.io/styleguide/go/ for Go files.
- Following https://protobuf.dev/programming-guides/style/ for proto files.
- TODO: Implement linter / pre-commits