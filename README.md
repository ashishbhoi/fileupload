# File Upload Server

A simple file upload server written in Go that allows you to upload, download, view, and manage files through a web interface.

## Features

- Upload files (max 50MB per file)
- View uploaded files
- Download files
- Delete multiple files
- Dark/Light theme support
- Responsive design

## Prerequisites

- Docker installed on your system
  - [Install Docker](https://docs.docker.com/get-docker/)

## Quick Start

1. Pull the Docker image:

```bash
docker pull ashishbhoi/fileupload:latest
```

2. Create a directory to store uploaded files:

```bash
mkdir uploads
```

3. Run the container:

```bash
docker run -d \
  --name fileupload \
  -p 8080:8080 \
  -v "$(pwd)/uploads:/uploads" \
  ashishbhoi/fileupload:latest
```

The application will be available at [http://localhost:8080](http://localhost:8080)

## Docker Compose

Alternatively, you can use Docker Compose. Create a `docker-compose.yml` file:

```yaml
version: "3.8"
services:
  fileupload:
    image: ashishbhoi/fileupload:latest
    container_name: fileupload
    ports:
      - "8080:8080"
    volumes:
      - ./uploads:/uploads
    restart: unless-stopped
```

Then run:

```bash
docker-compose up -d
```

## Configuration

The server uses the following default configuration:

- Port: 8080
- Maximum file size: 50MB
- Upload directory: /uploads

## Building from Source

If you want to build the image yourself:

1. Clone the repository:

```bash
git clone https://github.com/ashishbhoi/fileupload.git
cd fileupload
```

2. Build the Docker image:

```bash
docker build -t fileupload .
```

3. Run the container:

```bash
docker run -d \
  --name fileupload \
  -p 8080:8080 \
  -v "$(pwd)/uploads:/uploads" \
  fileupload
```

## License

This project is licensed under the GNU General Public License v3.0 - see the [LICENSE](LICENSE) file for details.

## Security

- All filenames are sanitized to prevent path traversal attacks
- Maximum file size is enforced
- Input validation is performed on all operations

## Support

If you encounter any issues or have questions, please file an issue on the GitHub repository.
