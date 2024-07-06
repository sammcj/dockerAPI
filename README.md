# DockerAPI

DockerAPI is a lightweight HTTP API for managing Docker containers and images, with optional support for Docker Compose profiles.

- [DockerAPI](#dockerapi)
  - [Features](#features)
  - [Requirements](#requirements)
  - [Installation](#installation)
    - [go install](#go-install)
    - [From Binary](#from-binary)
  - [Usage](#usage)
    - [Running the API](#running-the-api)
      - [Flags](#flags)
    - [API Endpoints](#api-endpoints)
    - [Example API Requests](#example-api-requests)
  - [Docker Deployment](#docker-deployment)
  - [Security Considerations](#security-considerations)
  - [Contributing](#contributing)
  - [License](#license)

## Features

- Container operations: restart, stop, start, remove
- Image operations: pull
- Docker Compose operations: pull, up, down, restart, stop, start
- Authentication via bearer token
- Configurable permissions for different operations
- JSON and pretty-printed output formats
- Logging with configurable levels

```shell
curl -X POST -H "Content-Type: application/json" -H "Authorization: Bearer your_token_here" -d '
{
  "operation": "pull",
  "service": "nginx",
  "profile": "nginx"
}
' http://localhost:9999/compose
{"message":"Operation pull completed successfully on service nginx"}
```

## Requirements

- Go 1.22 or later
- Docker
- Optional: Docker Compose (for Compose operations)

## Installation

### go install

```shell
go install github.com/sammcj/dockerAPI@HEAD
```

### From Binary

1. Download the latest release from the [Releases](https://github.com/sammcj/dockerAPI/releases) page.
2. Extract the tarball:

   ```shell
   tar -xvf dockerapi_*.tar.gz
   cd dockerapi
   ```

3. Run the application:

   ```shell
    ./dockerapi [flags]
    ```

### From Source

1. Clone the repository:

   ```shell
   git clone https://github.com/sammcj/dockerAPI.git
   cd dockerAPI
   ```

2. Build the application:

   ```shell
   make build
   ```

## Usage

### Running the API

```shell
./dockerapi [flags]
```

#### Flags

- `--auth-token`: Auth token for API requests
- `--allow-restart`: Allow container restart operation (default true)
- `--allow-stop`: Allow container stop operation (default true)
- `--allow-start`: Allow container start operation (default true)
- `--allow-remove`: Allow container remove operation (default false)
- `--allow-pull`: Allow image pull operation (default true)
- `--allow-compose`: Allow Docker Compose operations (default true)
- `--port`: Port to listen on (default 8080)
- `--log-level`: Log level (debug, info, warn, error) (default "info")
- `--compose-path`: Path to Docker Compose project (default "./")
- `-v`: Print the version and exit
- `--help-api`: Show usage examples

### API Endpoints

- `/container`: Container operations
- `/image`: Image operations
- `/compose`: Docker Compose operations

For detailed API usage examples, run `./dockerapi --help-api`.

### Example API Requests

For pretty-printed output, add ?format=pretty to the URL of any request, e.g. `http://localhost:8080/container?format=pretty`

Restart a container:

```shell
curl -X POST -H "Content-Type: application/json" -H "Authorization: Bearer your_token_here" \
 -d '
{
  "operation": "restart",
  "container": "my-container"
}
' \
 http://localhost:8080/container
```

Stop a container:

```shell
curl -X POST -H "Content-Type: application/json" -H "Authorization: Bearer your_token_here" \
 -d '
{
  "operation": "stop",
  "container": "my-container"
}
' \
 http://localhost:8080/container
```

Start a container:

```shell
curl -X POST -H "Content-Type: application/json" -H "Authorization: Bearer your_token_here" \
 -d '
{
  "operation": "start",
  "container": "my-container"
}
' \
 http://localhost:8080/container
```

Remove a stopped container:

```shell
curl -X POST -H "Content-Type: application/json" -H "Authorization: Bearer your_token_here" \
 -d '
{
  "operation": "remove",
  "container": "my-container"
}
' \
 http://localhost:8080/container
```

Pull an image:

```shell
curl -X POST -H "Content-Type: application/json" -H "Authorization: Bearer your_token_here" \
 -d '
{
  "operation": "pull",
  "image": "nginx:latest"
}
' \
 http://localhost:8080/image
```

Docker Compose - Restart a service:

```shell
curl -X POST -H "Content-Type: application/json" -H "Authorization: Bearer your_token_here" \
 -d '
{
  "operation": "restart",
  "service": "web",
  "profile": "development"
}
' \
 http://localhost:8080/compose
```

Docker Compose - Stop a service:

```shell
curl -X POST -H "Content-Type: application/json" -H "Authorization: Bearer your_token_here" \
 -d '
{
  "operation": "stop",
  "service": "web",
  "profile": "development"
}
' \
 http://localhost:8080/compose
```

Docker Compose - Start a service:

```shell
curl -X POST -H "Content-Type: application/json" -H "Authorization: Bearer your_token_here" \
 -d '
{
  "operation": "start",
  "service": "web",
  "profile": "development"
}
' \
 http://localhost:8080/compose
```

Docker Compose - Remove a service:

```shell
curl -X POST -H "Content-Type: application/json" -H "Authorization: Bearer your_token_here" \
 -d '
{
  "operation": "remove",
  "service": "web",
  "profile": "development"
}
' \
 http://localhost:8080/compose
```

Docker Compose - Pull images for a service:

```shell
curl -X POST -H "Content-Type: application/json" -H "Authorization: Bearer your_token_here" \
 -d '
{
  "operation": "pull",
  "service": "web",
  "profile": "development"
}
' \
 http://localhost:8080/compose
```

For pretty-printed output, add ?format=pretty to the URL, e.g. `http://localhost:8080/compose?format=pretty`

## Docker Deployment

A Dockerfile and docker-compose.yml are provided for easy deployment. To run DockerAPI in a container:

1. Build the Docker image:

   ```shell
   docker build -t dockerapi .
   ```

2. Run the container:

   ```shell
   docker run -d -p 8080:8080 -v /var/run/docker.sock:/var/run/docker.sock dockerapi
   ```

Alternatively, use Docker Compose (after editing the docker-compose.yml file):

```shell
docker-compose up -d
```

## Security Considerations

- Use HTTPS in production to secure API communications.
- Limit access to the Docker socket and DockerAPI.
  - You can use a docker socket proxy such as [Tecnativa/docker-socket-proxy](https://github.com/Tecnativa/docker-socket-proxy) to restrict access to the Docker socket and expose a secure API to DockerAPI.
- Regularly update dependencies and the base image.
- Use the principle of least privilege when configuring allowed operations.

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

- Copyright 2024 Sam McLeod
- This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
