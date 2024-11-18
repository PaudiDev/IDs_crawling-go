# Go Crawler 🚀

A powerful and lightweight products IDs crawler written in Go designed to efficiently scrape web data\
with customizable configuration options.

## Prerequisites 🛠️

Ensure you have **Docker** installed on your machine.\
This project relies on Docker to handle dependencies and environment isolation, so you do not have to install Go or any additional libraries locally.

## Setup 🐳

1. **Clone the repository**:
   ```bash
   git clone https://github.com/PaudiDev/IDs_crawling-go.git
   cd IDs_crawling-go/
   ```

2. **Create a `.env` file** in the project root directory with the following environment variables:
   ```env
   CONFIG_FILE=config.yml
   PROXIES_FILE=app/assets/proxies.txt
   USER_AGENTS_FILE=app/assets/user-agents.txt
   STATUS_LOG_FILE=log/status.log
   ```

   - If you are a developer working on this project locally, append `.local` to the file paths (except for `STATUS_LOG_FILE`, as .log files are in .gitignore anyway):

     ```env
     CONFIG_FILE=config.yml.local
     PROXIES_FILE=app/assets/proxies.txt.local
     USER_AGENTS_FILE=app/assets/user-agents.txt.local
     STATUS_LOG_FILE=log/status.log
     ```
    
     Also remember to create the new files in the same folders of the 'non-local' counterparts.

3. **Create a `logs/` directory** in the project root. This will store runtime log files.
   ```bash
   mkdir logs
   ```

## Usage 🏃

The project comes with convenient bash scripts to build and run the Docker container. 

### Available Scripts

Always run the scripts from the project root to ensure correct behaviour.

1. **Build the Docker image**:
   ```bash
   ./scripts/build.sh
   ```
   This script executes Docker's build command using the `DockerFile` located in the root directory.

2. **Run the Docker container**:
   ```bash
   ./scripts/run.sh
   ```
   This script runs the container using the Docker image previously built.

3. **Build and Run** (all-in-one):
   ```bash
   ./scripts/build_and_run.sh
   ```
   Combines the build and run steps in a single script for convenience.

## Folder Structure 📂

```plaintext
.
├── app/
│   ├── assets/
│   │   ├── proxies.txt          # Proxies list (duplicate as `.local` for development)
│   │   └── user-agents.txt      # User agents list (duplicate as `.local` for development)
│   ├── cmd/...
│   └── pkg/...
├── log/
│   ├── logs.log                 # Runtime crawling log (ignored by Git, automatically generated)
│   └── status.log               # Runtime status log (ignored by Git, automatically generated)
├── scripts/
│   ├── build.sh                 # Script to build the Docker image
│   ├── run.sh                   # Script to run the Docker container
│   └── build_and_run.sh         # Script to build and run the Docker container (convenient)
├── .env                         # Environment variables
├── config.yml                   # Config file (duplicate as `.local` for development)
├── DockerFile                   # Docker configuration
├── go.mod
├── go.sum
└── README.md
```

## Contribution 🤝

If you're contributing to the project:
- Use `.local` variants for config files and assets to avoid conflicts with the main configuration.
- Ensure all your changes are tested in the Dockerized environment.

---

Happy Crawling! 🕷️