# Personal Key-Value Store

## Overview

This project implements a personal key-value store using Go and gRPC. It supports various storage modes, including:

- **Non-Persistent Mode**: Data is stored only in memory (non-persistent).
- **Snapshot Mode**: Data is periodically saved to snapshot files.
- **Append-Only File (AOF) Mode**: All write operations are logged to disk.
- **Hybrid Mode**: Combines both Snapshot and AOF methods for persistence.

The server provides a gRPC interface for clients to interact with the key-value store.

The project includes:

- A gRPC server (`server/server.go`).
- Support for `string:bytes` data storage.
- An interactive client (`client/client_example.go`) that allows you to interact with the server using commands.
- Separate Dockerfiles for server and client.
- Instructions on how to build, run, and test the server and client using Docker Compose.

### Future Development ###
- Support for more data structures
- Implement distributed architecture with:
  - Consistent hashing
  - Virtual node replication
  - RAFT leader election

## Table of Contents

- [Prerequisites](#prerequisites)
- [Project Structure](#project-structure)
- [Docker Compose Deployment](#docker-compose-deployment)
  - [Building and Starting the Services](#building-and-starting-the-services)
  - [Verifying the Deployment](#verifying-the-deployment)
  - [Stopping the Services](#stopping-the-services)
- [Running Tests](#running-tests)
- [Using the Interactive Client](#using-the-interactive-client)
- [Writing Your Own Client](#writing-your-own-client)
- [Continuous Integration](#continuous-integration)
- [License](#license)
- [Style](#style)

## Prerequisites

- **Docker**: Ensure Docker is installed on your system. You can download it from the [official website](https://www.docker.com/get-started).
- **Git**: For cloning the repository.

## Project Structure
```plaintext
.
├── client/
│   ├── client_example.go
│   └── Dockerfile.client
├── server/
│   ├── server.go
│   └── Dockerfile.server
├── tests/
│   ├── server_test.go
│   ├── client_test.go
│   ├── Dockerfile.test
│   └── run_tests.sh
├── docker-compose.yml
├── go.mod
├── go.sum
└── README.md
```

## Docker Compose Deployment

### Building and Starting the Services

1. **Clone the Repository**

    ```bash
    git clone https://github.com/jry0/personal-kv-store.git
    cd personal-kv-store
    ```

2. **Start the Services with Docker Compose**

    Use Docker Compose to build the images and start the containers.

    ```bash
    docker-compose up --build -d
    ```

    - `--build`: Rebuilds the images if there are any changes.
    - `-d`: Runs the containers in detached mode.

### Verifying the Deployment

1. **Check Running Containers**

    Ensure that both the server and client containers are running.

    ```bash
    docker-compose ps
    ```

2. **View Server Logs**

    Inspect the server logs to ensure it's running correctly.

    ```bash
    docker-compose logs kvstore_server
    ```

3. **Access the Interactive Client**

    Attach to the client container to interact with the key-value store.

    ```bash
    docker exec -it kvstore_client ./kvstore_client
    ```

### Stopping the Services

To stop and remove the containers, networks, and volumes created by Docker Compose, run:

```bash
docker-compose down
```

---

## Running Tests

To ensure the reliability and correctness of your key-value store, it's essential to run both unit and integration tests.

### Prerequisites

- **Go**: Ensure Go is installed on your system for running unit tests.
- **Docker Compose**: Required for running integration tests within containers.

### Unit Tests

Unit tests are located in `tests/server_test.go` and `tests/client_test.go`. They cover the core functionalities of the server and client.

**Run Unit Tests**

Navigate to the project directory and execute:

```bash
go test -v ./...
```

This command runs all tests in the project with verbose output.

### Integration Tests

Integration tests verify the interaction between the server and client within a Docker environment.

**Run Integration Tests**

1. **Ensure Docker Compose is Up**

   Start the necessary services:

   ```bash
   docker-compose up -d kvserver
   ```

2. **Run Tests**

   Execute the test container which runs the integration tests:

   ```bash
   docker-compose run --rm test
   ```

3. **Stop Services**

   After testing, stop the services:

   ```bash
   docker-compose down
   ```

### Automated Test Script

For convenience, you can use the provided `tests/run_tests.sh` script to execute all tests sequentially.

**Run All Tests**

```bash
bash tests/run_tests.sh
```

This script performs the following actions:

1. Runs unit tests.
2. Starts the `kvserver` container.
3. Waits for the server to initialize.
4. Runs integration tests using the `test` service.
5. Stops and removes all containers.

---

## Continuous Integration

To automate testing on every push and pull request, set up GitHub Actions as follows:

1. **Create GitHub Actions Workflow Directory**

   ```bash
   mkdir -p .github/workflows
   ```

2. **Add Workflow File**

   Create a file `.github/workflows/ci.yml` with the following content:

   <file>
   ```yaml
   // filepath: /Users/jxing/side-projects/personal-kv-store/.github/workflows/ci.yml
   name: CI

   on:
     push:
       branches: [ main ]
     pull_request:
       branches: [ main ]

   jobs:
     build:

       runs-on: ubuntu-latest

       steps:
       - name: Checkout repository
         uses: actions/checkout@v3

       - name: Set up Go
         uses: actions/setup-go@v4
         with:
           go-version: 1.23

       - name: Install Protobuf Compiler
         run: sudo apt-get install -y protobuf-compiler

       - name: Install Protoc Plugins
         run: |
           go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
           go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
           export PATH=$PATH:$(go env GOPATH)/bin

       - name: Generate Protobuf Code
         run: |
           protoc \
             --go_out=./server \
             --go-grpc_out=./server \
             --go_opt=paths=source_relative \
             --go-grpc_opt=paths=source_relative \
             kvstore.proto

       - name: Run Unit Tests
         run: |
           cd tests
           go test -v ./...

       - name: Build and Test Docker Images
         run: |
           docker-compose build
           docker-compose up -d kvserver
           sleep 5
           docker-compose run --rm test
           docker-compose down
   ```
   </file>

3. **Commit and Push**

   ```bash
   git add .github/workflows/ci.yml
   git commit -m "Add GitHub Actions CI workflow"
   git push origin main
   ```

This workflow will:

- **Trigger** on pushes and pull requests to the `main` branch.
- **Set up Go** and install necessary dependencies.
- **Generate Protobuf code**.
- **Run unit tests** located in the `tests/` directory.
- **Build Docker images** and run integration tests using Docker Compose.

---

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

1. **Define the Protobuf Messages and Service**

2. **Use the kvstore.proto File as a Reference**

3. **Generate Client Code**

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

4. **Establish a Connection to the Server**

    *Connect to the server at kvstore_server:50051 (when using Docker network) or localhost:50051 (if running client locally).*

    *See [`client_example.go`](https://github.com/jry0/personal-kv-store/blob/main/client_example.go) as reference*

5. **Use the Client Stub to Call the Methods**

    *See [`client_example.go`](https://github.com/jry0/personal-kv-store/blob/main/client_example.go) as reference*

**For Other Languages:**

Generate client code using the respective Protobuf plugin (e.g., grpc_tools_node_protoc for Node.js, grpcio-tools for Python). Follow the language-specific conventions for creating stubs and making RPC calls.

## License

<!-- Add your license information here -->

## Style

- Following https://google.github.io/styleguide/go/ for Go files.
- Following https://protobuf.dev/programming-guides/style/ for proto files.
- TODO: Implement linter / pre-commits
