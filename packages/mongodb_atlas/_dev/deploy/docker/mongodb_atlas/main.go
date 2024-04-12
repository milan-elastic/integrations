package main

import (
	"compress/gzip"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

// Replace these values with your desired username and password
const (
	username = "admin"
	password = "MongoDB@123"
)

func generateDigest(username, realm, password, method, uri, nonce, nc, cnonce, qop string) string {
	ha1 := md5.New()
	fmt.Fprintf(ha1, "%s:%s:%s", username, realm, password)
	ha1Hex := fmt.Sprintf("%x", ha1.Sum(nil))

	ha2 := md5.New()
	fmt.Fprintf(ha2, "%s:%s", method, uri)
	ha2Hex := fmt.Sprintf("%x", ha2.Sum(nil))

	response := md5.New()
	fmt.Fprintf(response, "%s:%s:%s:%s:%s:%s", ha1Hex, nonce, nc, cnonce, qop, ha2Hex)
	return fmt.Sprintf("%x", response.Sum(nil))
}

func digestAuth(w http.ResponseWriter, r *http.Request, realm, nonce, qop string) bool {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		w.Header().Set("WWW-Authenticate", fmt.Sprintf(`Digest realm="%s", qop="%s", nonce="%s"`, realm, qop, nonce))
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return false
	}

	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || parts[0] != "Digest" {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return false
	}

	params := make(map[string]string)
	for _, param := range strings.Split(parts[1], ",") {
		kv := strings.SplitN(strings.TrimSpace(param), "=", 2)
		if len(kv) != 2 {
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return false
		}
		params[kv[0]] = strings.Trim(kv[1], `"`)
	}

	calculatedResponse := generateDigest(params["username"], realm, password, r.Method, r.RequestURI, nonce, params["nc"], params["cnonce"], params["qop"])

	if calculatedResponse != params["response"] {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return false
	}

	return true
}

func hostHandler(w http.ResponseWriter, r *http.Request) {

	realm := "MyRealm"
	nonce := "123456"
	qop := "auth"

	authorized := digestAuth(w, r, realm, nonce, qop)
	if !authorized {
		return
	}
	// Create the response
	type Link struct {
		Rel string `json:"rel"`
	}
	type Result struct {
		Hostname string `json:"hostname"`
	}
	type Response struct {
		Links   []Link   `json:"links"`
		Results []Result `json:"results"`
	}

	// Create the response
	response := Response{
		Links: []Link{
			{
				Rel: "self",
			},
		},
		Results: []Result{
			{
				Hostname: "hostname1",
			},
		},
	}

	// Set the content type to application/json
	w.Header().Set("Content-Type", "application/vnd.atlas.2023-02-01+json")

	// Encode and send the response
	err := json.NewEncoder(w).Encode(response)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error encoding JSON: %v", err), http.StatusInternalServerError)
		return
	}
}

func handler(w http.ResponseWriter, r *http.Request) {
	// Digest Authentication
	realm := "MyRealm"
	nonce := "123456"
	qop := "auth"

	authorized := digestAuth(w, r, realm, nonce, qop)
	if !authorized {
		return
	}

	fileName := strings.TrimPrefix(r.URL.Path, "/api/atlas/v2/groups/mongodb-group1/clusters/hostname1/logs/")
	// Replace "path/to/your/log/file.log" with the actual path to your log file
	var logFilePath string
	if fileName == "mongodb.gz" {
		logFilePath = "mongod_database_data.log"
	}

	// Open the log file
	logFile, err := os.Open(logFilePath)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error opening log file: %v", err), http.StatusInternalServerError)
		return
	}
	defer logFile.Close()

	// Create a gzip writer for the response
	w.Header().Set("Content-Type", "application/vnd.atlas.2023-02-01+gzip")
	w.Header().Set("Content-Disposition", `attachment; filename="mongodb-log.gz"`)

	gzipWriter := gzip.NewWriter(w)
	defer gzipWriter.Close()

	// Copy the log file content to the gzip writer
	_, err = io.Copy(gzipWriter, logFile)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error reading log file: %v", err), http.StatusInternalServerError)
		return
	}
}

func main() {
	// Existing handler for serving the compressed file
	http.HandleFunc("/api/atlas/v2/groups/mongodb-group1/clusters/hostname1/logs/mongodb.gz", handler)

	// New handler for returning the hostname
	http.HandleFunc("/api/atlas/v2/groups/mongodb-group1/processes", hostHandler)

	// Start the server
	http.ListenAndServe(":7780", nil)
}
