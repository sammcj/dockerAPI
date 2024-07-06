package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/spf13/pflag"

	"github.com/fatih/color"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/sirupsen/logrus"
)

var (
	logger       *logrus.Logger
	dockerClient *client.Client
)

// Config holds the application configuration
type Config struct {
	AuthToken          string
	AllowRestart       bool
	AllowStop          bool
	AllowStart         bool
	AllowRemove        bool
	AllowPull          bool
	AllowComposeOps    bool
	Port               int
	LogLevel           string
	ComposeProjectPath string
}

// AppError represents an application-specific error
type AppError struct {
	Message string
	Code    int
}

func (e *AppError) Error() string {
	return e.Message
}

var Version string // Version is set by the build system

// main function with improved setup and error handling

func main() {
	if Version == "" {
		Version = ""
	}

	config, err := loadConfig()
	if err != nil {
		fmt.Printf("Error loading configuration: %v\n", err)
		os.Exit(1)
	}

	logger, err = initLogger(config.LogLevel)
	if err != nil {
		log.Fatalf("Failed to initialise logger: %v", err)
	}

	dockerClient, err = client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		logger.Fatalf("Failed to create Docker client: %v", err)
	}

	http.HandleFunc("/container", authMiddleware(func(w http.ResponseWriter, r *http.Request) {
		handleContainerOperation(w, r, config)
	}, config))
	http.HandleFunc("/image", authMiddleware(func(w http.ResponseWriter, r *http.Request) {
		handleImageOperation(w, r, config)
	}, config))
	http.HandleFunc("/compose", authMiddleware(func(w http.ResponseWriter, r *http.Request) {
		handleComposeOperation(w, r, config)
	}, config))

	addr := fmt.Sprintf(":%d", config.Port)
	logger.Infof("Starting DockerAPI on %s", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		logger.Fatalf("Failed to start server: %v", err)
	}
}

func handleContainerOperation(w http.ResponseWriter, r *http.Request, config *Config) {
	var req struct {
		Operation string `json:"operation"`
		Container string `json:"container"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, &AppError{Message: "Invalid request body", Code: http.StatusBadRequest}, r)
		return
	}

	if req.Container == "" {
		respondWithError(w, &AppError{Message: "Container name is required", Code: http.StatusBadRequest}, r)
		return
	}

	ctx := r.Context()

	switch req.Operation {
	case "restart":
		if !config.AllowRestart {
			respondWithError(w, &AppError{Message: "Restart operation not allowed", Code: http.StatusForbidden}, r)
			return
		}
		if err := dockerClient.ContainerRestart(ctx, req.Container, container.StopOptions{}); err != nil {
			logger.WithFields(logrus.Fields{
				"container": req.Container,
				"error":     err,
			}).Error("Failed to restart container")
			respondWithError(w, &AppError{Message: fmt.Sprintf("Failed to restart container: %v", err), Code: http.StatusInternalServerError}, r)
			return
		}
		logger.WithField("container", req.Container).Info("Container restarted")

	case "stop":
		if !config.AllowStop {
			respondWithError(w, &AppError{Message: "Stop operation not allowed", Code: http.StatusForbidden}, r)
			return
		}
		if err := dockerClient.ContainerStop(ctx, req.Container, container.StopOptions{}); err != nil {
			logger.Errorf("Failed to stop container %s: %v", req.Container, err)
			respondWithError(w, &AppError{Message: fmt.Sprintf("Failed to stop container: %v", err), Code: http.StatusInternalServerError}, r)
			return
		}
		logger.Infof("Container %s stopped", req.Container)

	case "start":
		if !config.AllowStart {
			respondWithError(w, &AppError{Message: "Start operation not allowed", Code: http.StatusForbidden}, r)
			return
		}
		if err := dockerClient.ContainerStart(ctx, req.Container, container.StartOptions{}); err != nil {
			logger.Errorf("Failed to start container %s: %v", req.Container, err)
			respondWithError(w, &AppError{Message: fmt.Sprintf("Failed to start container: %v", err), Code: http.StatusInternalServerError}, r)
			return
		}
		logger.Infof("Container %s started", req.Container)

	case "remove":
		if !config.AllowRemove {
			respondWithError(w, &AppError{Message: "Remove operation not allowed", Code: http.StatusForbidden}, r)
			return
		}
		if err := dockerClient.ContainerRemove(ctx, req.Container, container.RemoveOptions{Force: true}); err != nil {
			logger.Errorf("Failed to remove container %s: %v", req.Container, err)
			respondWithError(w, &AppError{Message: fmt.Sprintf("Failed to remove container: %v", err), Code: http.StatusInternalServerError}, r)
			return
		}

		logger.Infof("Container %s removed", req.Container)

	default:
		respondWithError(w, &AppError{Message: "Invalid operation", Code: http.StatusBadRequest}, r)
		return
	}

	respondWithMessage(w, http.StatusOK, fmt.Sprintf("Operation %s completed successfully on container %s", req.Operation, req.Container), r)
}

// initLogger initialises the logger with the specified log level
func initLogger(logLevel string) (*logrus.Logger, error) {
	logger := logrus.New()
	level, err := logrus.ParseLevel(logLevel)
	if err != nil {
		return nil, fmt.Errorf("invalid log level: %v", err)
	}
	logger.SetLevel(level)
	logger.SetFormatter(&logrus.JSONFormatter{})
	return logger, nil
}

func loadConfig() (*Config, error) {
	config := &Config{
		AuthToken:          os.Getenv("AUTH_TOKEN"),
		AllowRestart:       getEnvBool("ALLOW_RESTART", true),
		AllowStop:          getEnvBool("ALLOW_STOP", true),
		AllowStart:         getEnvBool("ALLOW_START", true),
		AllowRemove:        getEnvBool("ALLOW_REMOVE", false),
		AllowPull:          getEnvBool("ALLOW_PULL", true),
		AllowComposeOps:    getEnvBool("ALLOW_COMPOSE", true),
		Port:               getEnvInt("PORT", 8080),
		LogLevel:           getEnvString("LOG_LEVEL", "info"),
		ComposeProjectPath: getEnvString("COMPOSE_PATH", "./"),
	}

	// Define flags
	pflag.StringVar(&config.AuthToken, "auth-token", config.AuthToken, "Auth token for API requests")
	pflag.BoolVar(&config.AllowRestart, "allow-restart", config.AllowRestart, "Allow container restart operation")
	pflag.BoolVar(&config.AllowStop, "allow-stop", config.AllowStop, "Allow container stop operation")
	pflag.BoolVar(&config.AllowStart, "allow-start", config.AllowStart, "Allow container start operation")
	pflag.BoolVar(&config.AllowRemove, "allow-remove", config.AllowRemove, "Allow container remove operation")
	pflag.BoolVar(&config.AllowPull, "allow-pull", config.AllowPull, "Allow image pull operation")
	pflag.BoolVar(&config.AllowComposeOps, "allow-compose", config.AllowComposeOps, "Allow Docker Compose operations")
	pflag.IntVar(&config.Port, "port", config.Port, "Port to listen on")
	pflag.StringVar(&config.LogLevel, "log-level", config.LogLevel, "Log level (debug, info, warn, error)")
	pflag.StringVar(&config.ComposeProjectPath, "compose-path", config.ComposeProjectPath, "Path to Docker Compose project")
	versionFlag := pflag.Bool("v", false, "Print the version and exit")
	helpApi := pflag.Bool("help-api", false, "Show usage examples")

	// Parse flags for both - and -- formats
	pflag.CommandLine.SetNormalizeFunc(func(f *pflag.FlagSet, name string) pflag.NormalizedName {
		return pflag.NormalizedName(strings.ReplaceAll(name, "_", "-"))
	})
	pflag.Parse()

	// Check if help flag is set
	if *helpApi {
		printAPIUsageExamples(config)
		os.Exit(0)
	}

	if *versionFlag {
		fmt.Println(Version)
		os.Exit(0)
	}

	// Generate a random auth token if not provided
	if config.AuthToken == "" {
		token, err := generateRandomToken(32)
		if err != nil {
			return nil, fmt.Errorf("failed to generate random auth token: %w", err)
		}
		fmt.Printf("Generated random auth token (WARNING: this will change each time you run the app!): %s\n", token)
		config.AuthToken = token
	}

	// Output the configured allowed operations
	fmt.Printf("Allowed operations: restart=%t, stop=%t, start=%t, remove=%t, pull=%t, compose=%t\n",
		config.AllowRestart, config.AllowStop, config.AllowStart, config.AllowRemove, config.AllowPull, config.AllowComposeOps)

	return config, nil
}
func handleImageOperation(w http.ResponseWriter, r *http.Request, config *Config) {
	var req struct {
		Operation string `json:"operation"`
		Image     string `json:"image"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, &AppError{Message: "Invalid request body", Code: http.StatusBadRequest}, r)
		return
	}

	ctx := context.Background()

	switch req.Operation {
	case "pull":
		if !config.AllowPull {
			respondWithError(w, &AppError{Message: "Pull operation not allowed", Code: http.StatusForbidden}, r)
			return
		}
		reader, err := dockerClient.ImagePull(ctx, req.Image, image.PullOptions{})
		if err != nil {
			logger.Errorf("Failed to pull image %s: %v", req.Image, err)
			respondWithError(w, &AppError{Message: fmt.Sprintf("Failed to pull image: %v", err), Code: http.StatusInternalServerError}, r)
			return
		}
		defer reader.Close()

		format := r.URL.Query().Get("format")
		if format == "pretty" {
			w.Header().Set("Content-Type", "text/plain")
			decoder := json.NewDecoder(reader)
			for {
				var message map[string]interface{}
				if err := decoder.Decode(&message); err != nil {
					if err == io.EOF {
						break
					}
					logger.Errorf("Error decoding Docker API response: %v", err)
					break
				}
				status, _ := message["status"].(string)
				id, _ := message["id"].(string)
				if status != "" {
					if id != "" {
						fmt.Fprintf(w, "%s: %s\n", status, id)
					} else {
						fmt.Fprintf(w, "%s\n", status)
					}
				}
				if progress, ok := message["progress"].(string); ok {
					fmt.Fprintf(w, "Progress: %s\n", progress)
				}
			}
		} else {
			w.Header().Set("Content-Type", "application/json")
			if _, err := io.Copy(w, reader); err != nil {
				logger.Errorf("Failed to stream pull output: %v", err)
				return
			}
		}
		logger.Infof("Image %s pulled", req.Image)

	default:
		respondWithError(w, &AppError{Message: "Invalid operation", Code: http.StatusBadRequest}, r)
		return
	}
}

func handleComposeOperation(w http.ResponseWriter, r *http.Request, config *Config) {
	if !config.AllowComposeOps {
		respondWithError(w, &AppError{Message: "Compose operations not allowed", Code: http.StatusForbidden}, r)
		return
	}

	var req struct {
		Operation string `json:"operation"`
		Service   string `json:"service"`
		Profile   string `json:"profile"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, &AppError{Message: "Invalid request body", Code: http.StatusBadRequest}, r)
		return
	}

	ctx := r.Context()

	switch req.Operation {
	case "pull", "up", "down", "restart", "stop", "start":
		if err := performComposeOperation(ctx, config.ComposeProjectPath, req.Operation, req.Service, req.Profile); err != nil {
			logger.Errorf("Failed to perform %s operation on service %s: %v", req.Operation, req.Service, err)
			respondWithError(w, &AppError{Message: fmt.Sprintf("Failed to perform operation: %v", err), Code: http.StatusInternalServerError}, r)
			return
		}
		logger.Infof("Operation %s completed successfully on service %s", req.Operation, req.Service)

	default:
		respondWithError(w, &AppError{Message: "Invalid operation", Code: http.StatusBadRequest}, r)
		return
	}

	respondWithMessage(w, http.StatusOK, fmt.Sprintf("Operation %s completed successfully on service %s", req.Operation, req.Service), r)
}

func performComposeOperation(ctx context.Context, projectPath, operation, service, profile string) error {
	// The projectPath is a directory that could contain any number of docker*compose*.y*ml files, we simply need to set the current working directory to this path
	if err := os.Chdir(projectPath); err != nil {
		return fmt.Errorf("failed to change directory to %s: %w", projectPath, err)
	}

	args := []string{"compose"}

	if profile != "" {
		args = append(args, "--profile", profile)
	}

	args = append(args, operation)

	if service != "" {
		args = append(args, service)
	}

	cmd := exec.CommandContext(ctx, "docker", args...)
	output, err := cmd.CombinedOutput()

	if err != nil {
		return fmt.Errorf("docker compose %s failed: %w\nOutput: %s", operation, err, string(output))
	}

	logger.Infof("docker compose %s completed successfully for service %s", operation, service)
	logger.Debugf("Command output: %s", string(output))

	return nil
}

// respondWithError sends a JSON or pretty-printed error response
func respondWithError(w http.ResponseWriter, err *AppError, r *http.Request) {
	format := r.URL.Query().Get("format")
	if format == "pretty" {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(err.Code)
		fmt.Fprintf(w, "Error: %s\n", err.Message)
	} else {
		respondWithJSON(w, err.Code, map[string]string{"error": err.Message})
	}
}

// respondWithJSON sends a JSON response
func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	response, _ := json.Marshal(payload)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(response)
}

// respondWithMessage sends a JSON or pretty-printed message response
func respondWithMessage(w http.ResponseWriter, code int, message string, r *http.Request) {
	format := r.URL.Query().Get("format")
	if format == "pretty" {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(code)
		fmt.Fprintln(w, message)
	} else {
		respondWithJSON(w, code, map[string]string{"message": message})
	}
}

// authMiddleware is a middleware function to handle authentication
func authMiddleware(next http.HandlerFunc, config *Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if config.AuthToken == "" {
			next.ServeHTTP(w, r)
			return
		}

		token := r.Header.Get("Authorization")
		if token == "" {
			respondWithError(w, &AppError{Message: "Missing authorization token", Code: http.StatusUnauthorized}, r)
			logger.Warn("Missing authorization token from: ", r.RemoteAddr)
			return
		}

		if token != fmt.Sprintf("Bearer %s", config.AuthToken) {
			respondWithError(w, &AppError{Message: "Invalid authorization token", Code: http.StatusUnauthorized}, r)
			logger.Warn("Invalid authorization token from: ", r.RemoteAddr)
			return
		}

		next.ServeHTTP(w, r)
	}
}

func getEnvBool(key string, defaultValue bool) bool {
	if value, exists := os.LookupEnv(key); exists {
		return value == "true"
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value, exists := os.LookupEnv(key); exists {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getEnvString(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

func generateRandomToken(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(bytes)[:length], nil
}

func printAPIUsageExamples(config *Config) {
	fmt.Println("DockerAPI API Usage Examples:")
	fmt.Println("-----------------------------------")

	printExample(config, "Restart a container", "/container", `{"operation":"restart","container":"my-container"}`)
	printExample(config, "Stop a container", "/container", `{"operation":"stop","container":"my-container"}`)
	printExample(config, "Start a container", "/container", `{"operation":"start","container":"my-container"}`)
	printExample(config, "Remove a container", "/container", `{"operation":"remove","container":"my-container"}`)
	printExample(config, "Pull an image", "/image", `{"operation":"pull","image":"nginx:latest"}`)
	printExample(config, "Docker Compose - Restart a service", "/compose", `{"operation":"restart","service":"web","profile":"development"}`)
	printExample(config, "Docker Compose - Stop a service", "/compose", `{"operation":"stop","service":"web","profile":"development"}`)
	printExample(config, "Docker Compose - Start a service", "/compose", `{"operation":"start","service":"web","profile":"development"}`)
	printExample(config, "Docker Compose - Remove a service", "/compose", `{"operation":"remove","service":"web","profile":"development"}`)
	printExample(config, "Docker Compose - Pull images for a service", "/compose", `{"operation":"pull","service":"web","profile":"development"}`)
}

func colouriseJSON(jsonString string) string {
	var data interface{}
	err := json.Unmarshal([]byte(jsonString), &data)
	if err != nil {
		return jsonString // Return original string if parsing fails
	}

	keyColor := color.New(color.FgBlue).SprintFunc()
	stringColor := color.New(color.FgGreen).SprintFunc()
	numberColor := color.New(color.FgYellow).SprintFunc()
	boolColor := color.New(color.FgCyan).SprintFunc()
	nullColor := color.New(color.FgRed).SprintFunc()

	var colourise func(v interface{}, depth int) string
	colourise = func(v interface{}, depth int) string {
		indent := strings.Repeat("  ", depth)
		switch vv := v.(type) {
		case string:
			return stringColor(fmt.Sprintf("%q", vv))
		case float64:
			return numberColor(fmt.Sprintf("%v", vv))
		case bool:
			return boolColor(fmt.Sprintf("%v", vv))
		case nil:
			return nullColor("null")
		case []interface{}:
			var elements []string
			for _, e := range vv {
				elements = append(elements, colourise(e, depth+1))
			}
			if len(elements) == 0 {
				return "[]"
			}
			return fmt.Sprintf("[\n%s%s,\n%s]", indent+"  ", strings.Join(elements, ",\n"+indent+"  "), indent)
		case map[string]interface{}:
			var pairs []string
			for k, v := range vv {
				pairs = append(pairs, fmt.Sprintf("%s: %s", keyColor(fmt.Sprintf("%q", k)), colourise(v, depth+1)))
			}
			if len(pairs) == 0 {
				return "{}"
			}
			return fmt.Sprintf("{\n%s%s\n%s}", indent+"  ", strings.Join(pairs, ",\n"+indent+"  "), indent)
		default:
			return fmt.Sprintf("%v", vv)
		}
	}

	return colourise(data, 0)
}

func printExample(config *Config, description, endpoint, jsonData string) {
	fmt.Printf("\n%s:\n", description)
	fmt.Printf("curl -X POST -H \"Content-Type: application/json\" -H \"Authorization: Bearer %s\" \\\n", config.AuthToken)
	fmt.Printf(" -d '\n%s\n' \\\n", colouriseJSON(jsonData))
	fmt.Printf(" http://localhost:%d%s\n", config.Port, endpoint)
	fmt.Printf("\nFor pretty-printed output, add ?format=pretty to the URL:\n")
	fmt.Printf(" http://localhost:%d%s?format=pretty\n", config.Port, endpoint)
}
