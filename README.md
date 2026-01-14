# Condenser
<p>
  <img src="assets/condenser_icon.png" alt="Project Icon" width="190">
</p>
Condenser is one of the components of the Raind container runtime stack and serves as the high-level container runtime.  
It is responsible for container lifecycle management, image management, and providing a REST API for external control.  
Condenser orchestrates container operations by invoking the low-level runtime Droplet.  

## Runtime Stack Architecture
The Raind container runtime stack is composed of three layers:

- Raind CLI – A user interface for operating the entire runtime stack
- Condenser – A high-level container runtime responsible for container lifecycle and image management (this repository)
- Droplet – A low-level container runtime that handles container startup, deletion, and related operations  
  (repository: https://github.com/pyxgun/Droplet)

Condenser acts as the control plane of the Raind stack.
It translates high-level API requests into concrete container operations by generating OCI specifications, managing container state, and delegating execution to Droplet.

## Features
Condenser currently supports:

- Container lifecycle management
    - Create, start, stop, delete, and inspect containers
    - Coordination with Droplet for low-level container execution

- Image management
    - Pulling container images from Docker Hub
    - Managing image layers and extracted root filesystems

- REST API
    - HTTP-based interface for controlling containers and images
    - Designed to be consumed by Raind CLI or external tools

## Build
Requirements

- Linux kernel with namespace & cgroup support
- Go (version 1.25 or later)
- root privileges (or appropriate capabilities)

```bash
git clone https://github.com/your-org/condenser.git
cd condenser
./scripts/build.sh
```

## Usage

Condenser is designed to run as a long-lived service and to be accessed via its REST API, typically from the Raind CLI.

### Start Condenser
```bash
sudo ./bin/condenser
```

By default, Condenser starts an HTTP server and waits for API requests to manage containers and images.

## Typical Workflow
A typical container startup sequence in the Raind stack looks like this:

- A client (Raind CLI or external tool) sends a request to Condenser via the REST API
- Condenser pulls the required image from Docker Hub (if not already present)
- Condenser generates an OCI-compliant config.json and prepares the container bundle
- Condenser invokes Droplet to create and start the container
- Condenser tracks container state and exposes it via the API

## Status
Condenser and the Raind container runtime stack are currently under active development.  
APIs, storage formats, and behavior may change without notice.