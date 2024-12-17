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

4. **Modify the `config.yml` file settings** based on your needs\

   The `config.yml` file is organized into three main sections: `core`, `http`, and `standard`.\
   Below is a detailed explanation of each section and its parameters.

   ---

   ### **1. Core Configuration (`core`)**

   The `core` section controls the general crawling behavior, focusing on concurrency settings.

   ```yaml
   core:
      max_concurrency:
      initial_concurrency:
      initial_step:
   ```

   - **`max_concurrency`**: The maximum number of concurrent requests allowed.
   - **`initial_concurrency`**: The initial number of concurrent requests (before first adjustment).
   - **`initial_step`**: The initial step which the current ID is incremented by (before first adjustment).

   ---

   ### **2. HTTP Configuration (`http`)**

   The `http` section manages the HTTP request settings and rate-limiting behavior.

   #### **General HTTP Settings**

   ```yaml
   http:
      requests_timeout_seconds:
      cookies_refresh_delay:
      max_rate_limits_per_second:
      rate_limit_wait_seconds:
   ```

   - **`requests_timeout_seconds`**: Timeout for HTTP requests, in seconds.
   - **`cookies_refresh_delay`**: Time in seconds before refreshing cookies.
   - **`max_rate_limits_per_second`**: Maximum allowed requests per second before rate limiting is triggered.
   - **`rate_limit_wait_seconds`**: Time to wait before retrying after hitting the rate limit.

   #### **Step Adjustment Settings (`step_data`)**

   ```yaml
      step_data:
         min_time_since_last_adjustment_milli:
         max_error_deviation:
         max_consecutive_errors:
         max_retries:
         max_time:
         aggressive_time:
         medium_time:
         min_time:
         retry_time:
         last_delay_offset:
   ```

   - **`min_time_since_last_adjustment_milli`**: Minimum time in milliseconds before another adjustment to the step can be made.
   - **`max_error_deviation`**: Maximum allowed deviation in errors (errors - successes) before retrying on the same ID if the last fetched item delay is lower than `retry_time`.
   - **`max_consecutive_errors`**: Number of consecutive errors allowed before setting the step to a negative value.
   - **`max_retries`**: Maximum amount of retries on the same ID (after hitting this the step will be set back to 1).
   - **`max_time`, `aggressive_time`, `medium_time`, `min_time`**: Various thresholds for adjusting the step based on the fetch delay of the last fetched item.
   - **`retry_time`**: Time to wait before retrying a failed request.
   - **`last_delay_offset`**: Determines the threshold for adjusting the step based on the difference between the last fetched item delay and the last best delay. If the difference exceeds this offset, the step decreases; if it is below the negative offset, the step increases.

      Always remember that a max step is calculated on each step adjustment, so that the step can never exceed a set value.

   #### **Concurrency Adjustment Settings (`concurrency_data`)**

   ```yaml
      concurrency_data:
         min_time_since_last_adjustment_milli:
         max_error_deviation:
         max_consecutive_errors:
         min_concurrency:
         max_time:
         medium_time:
         min_time:
   ```

   - **`min_time_since_last_adjustment_milli`**: Minimum time in milliseconds before another adjustment to the concurrency can be made.
   - **`max_error_deviation`**: Maximum allowed deviation in errors (errors - successes) before drastically decreasing the concurrency.
   - **`max_consecutive_errors`**: Number of consecutive errors allowed before setting the concurrency to `min_concurrency`.
   - **`min_concurrency`**: Minimum allowed concurrency level.
   - **`max_time`, `medium_time`, `min_time`**: Various thresholds for adjusting the concurrency based on the fetch delay of the last fetched item.

   ---

   ### **3. Standard Configuration (`standard`)**

   The `standard` section defines requests URLs, response handling, and other project-specific settings that hardly change.
   ```yaml
   standard:
   ```

   #### **URLs settings (`urls`)**

   **IMPORTANT**:\
   `items_url` and `item_url` requests are expected to return JSON. 

   ```yaml
      urls:
         base_url:
         items_url:
         item_url:
         item_url_after_id:
         randomize_item_url_addition: true
   ```

   - **`base_url`**: Base URL of the website to crawl (cookies will be fetched from here).
   - **`items_url`**: URL to get the last published items (the last item ID will be taken from here; the crawler will start with this ID).
   - **`item_url`**: URL to get details about an item based on its ID (the ID will be appended to it dynamically)
   - **`item_url_after_id`**: Last part of the item url (will be appended after the ID in `item_url`). If none is expected, simply set this value to `""`
   - **`randomize_item_url_addition`**: If set to true, `item_url_after_id` will be appended randomly; if set to false, it will always be appended

   #### **Items Response**

   ```yaml
      items_response:
         items:
         id:
   ```

   - **`items`**: Key in the JSON response of the `items_url` request containing a list of items.
   - **`id`**: Key in each item specifying its unique ID.

   #### **Item Response**

   ```yaml
      item_response:
         item:
         timestamp:
   ```

   - **`item`**: Key in the JSON response containing the item details.
   - **`timestamp`**: Key specifying the timestamp field in the item details.

   #### **WebSocket**

   ```yaml
      websocket:
         ws_url:
         ws_headers:
            header_name1:
            header_name2:
            header_name3:
   ```

   - **`ws_url`**: The URL of the websocket server that will receive the requests response to `item_url` in JSON format (knowledge of websocket is expected). If the websocket server is hosted on the same machine that host the docker container, but not in a docker container, you can use 'host.docker.internal' for the host part instead of the machine private IP.
   - **`ws_headers`**: Additional headers you want to be sent to the websocket server in order to allow the connection. The `header_name<x>` keys are simple placeholders. You can modify them, their amount, set multiple values for the same key and leave this setting empty if you do not need to send additional headers.

   #### **Other Settings**

   ```yaml
      session_cookie_name:
      timestamp_format:
      initial_delay:
   ```

   - **`session_cookie_name`**: Name of the session cookie used for requests. This cookie must always be present in the client cookies. If this condition is ever not satisfied the program is intended to stop.
   - **`timestamp_format`**: Format of the timestamp in the item details.
   - **`initial_delay`**: Initial value of the variable that keeps track of the last best delay of an item. Set this value so that it is much higher than the average highest delay of any possible item.

   ---

   ## Example

   The file is already setup with an example configuration. Feel free to adjust it as you need!

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