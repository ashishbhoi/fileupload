package main

import (
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"net/url" // Needed for URL encoding messages
	"os"
	"path/filepath"
	"strings" // Needed for joining error messages
	// "time" // Not strictly needed anymore unless logging timestamps explicitly
)

const (
	uploadDir     = "./uploads" // Directory to store uploaded files
	templateDir   = "./templates"
	port          = "8080"     // Port to run the server on
	maxUploadSize = 10 << 20 // 10 MB max upload size
)

// Data structure to pass to the HTML template
type TemplateData struct {
	Files   []string
	Message string // For success/error messages
	Error   string
}

var tmpl *template.Template // Global template cache

func main() {
	var err error
	// Ensure upload directory exists
	err = os.MkdirAll(uploadDir, os.ModePerm)
	if err != nil {
		log.Fatalf("Could not create upload directory: %v", err)
	}

	// Parse templates once at startup
	tmpl, err = template.ParseGlob(filepath.Join(templateDir, "*.html"))
	if err != nil {
		log.Fatalf("Could not parse templates: %v", err)
	}

	// --- Routing ---

	// Handle the root page (list files and show upload form)
	http.HandleFunc("/", indexHandler)

	// Handle file uploads
	http.HandleFunc("/upload", uploadHandler)

	// Handle multiple file deletions (NEW)
	http.HandleFunc("/delete-multiple", deleteMultipleHandler)

	// Handle single file deletions (Optional - kept for now, but UI doesn't use it)
	http.HandleFunc("/delete", deleteHandler)

	// Serve static files (uploaded files) for download
	// Strip '/files/' prefix so it maps correctly to 'uploadDir'
	fs := http.FileServer(http.Dir(uploadDir))
	http.Handle("/files/", http.StripPrefix("/files/", fs))

	// --- Start Server ---
	log.Printf("Server starting on http://localhost:%s\n", port)
	err = http.ListenAndServe(":"+port, nil)
	if err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}

// indexHandler serves the main page with file list and upload form
func indexHandler(w http.ResponseWriter, r *http.Request) {
	files, err := listFiles(uploadDir)
	if err != nil {
		log.Printf("Error listing files: %v", err)
		http.Error(w, "Could not list files", http.StatusInternalServerError)
		return
	}

	// Get potential messages from query params (after redirects)
	message := r.URL.Query().Get("message")
	errorMsg := r.URL.Query().Get("error")

	data := TemplateData{
		Files:   files,
		Message: message,
		Error:   errorMsg,
	}

	err = tmpl.ExecuteTemplate(w, "index.html", data)
	if err != nil {
		log.Printf("Error executing template: %v", err)
		http.Error(w, "Could not render page", http.StatusInternalServerError)
	}
}

// uploadHandler handles file uploads via POST request
func uploadHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Enforce maximum upload size
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)
	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		log.Printf("Error parsing multipart form: %v", err)
		// Redirect with URL encoded error message
		redirectURL := "/?error=" + url.QueryEscape("File too large (max 10MB)")
		http.Redirect(w, r, redirectURL, http.StatusSeeOther)
		return
	}

	file, handler, err := r.FormFile("file") // "file" is the name attribute in the form
	if err != nil {
		log.Printf("Error retrieving file from form: %v", err)
		redirectURL := "/?error=" + url.QueryEscape("Invalid file upload")
		http.Redirect(w, r, redirectURL, http.StatusSeeOther)
		return
	}
	defer file.Close()

	// Sanitize filename (important for security)
	// We only use the base name to prevent path traversal attacks
	filename := filepath.Base(handler.Filename)
	if filename == "." || filename == ".." || filename == "" { // Also check for empty
		redirectURL := "/?error=" + url.QueryEscape("Invalid filename")
		http.Redirect(w, r, redirectURL, http.StatusSeeOther)
		return
	}

	// Create the destination file path
	dstPath := filepath.Join(uploadDir, filename)

	// Create the destination file on the server
	dst, err := os.Create(dstPath)
	if err != nil {
		log.Printf("Error creating file on server: %v", err)
		redirectURL := "/?error=" + url.QueryEscape("Could not save file")
		http.Redirect(w, r, redirectURL, http.StatusSeeOther)
		return
	}
	defer dst.Close()

	// Copy the uploaded file data to the destination file
	_, err = io.Copy(dst, file)
	if err != nil {
		log.Printf("Error copying file data: %v", err)
		// Attempt to remove partially uploaded file
		os.Remove(dstPath)
		redirectURL := "/?error=" + url.QueryEscape("Could not copy file data")
		http.Redirect(w, r, redirectURL, http.StatusSeeOther)
		return
	}

	log.Printf("Successfully uploaded file: %s\n", filename)
	// Redirect with URL encoded success message
	redirectURL := "/?message=" + url.QueryEscape(fmt.Sprintf("File '%s' uploaded successfully", filename))
	http.Redirect(w, r, redirectURL, http.StatusSeeOther)
}

// deleteMultipleHandler handles deletion of multiple files via POST request (NEW)
func deleteMultipleHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse form data to get the checked filenames
	if err := r.ParseForm(); err != nil {
		log.Printf("Error parsing delete form: %v", err)
		redirectURL := "/?error=" + url.QueryEscape("Error processing delete request")
		http.Redirect(w, r, redirectURL, http.StatusSeeOther)
		return
	}

	// Get the slice of filenames from the form (key is "filenames" from checkboxes)
	filenamesToDelete := r.Form["filenames"]

	if len(filenamesToDelete) == 0 {
		redirectURL := "/?error=" + url.QueryEscape("No files selected for deletion")
		http.Redirect(w, r, redirectURL, http.StatusSeeOther)
		return
	}

	deletedCount := 0
	errorMessages := []string{} // Store any errors encountered

	for _, filename := range filenamesToDelete {
		// *** Security Crucial: Sanitize each filename ***
		baseFilename := filepath.Base(filename)
		if baseFilename == "." || baseFilename == ".." || baseFilename != filename {
			log.Printf("Attempted deletion with invalid filename: %s", filename)
			errorMessages = append(errorMessages, fmt.Sprintf("Invalid filename skipped: %s", filename))
			continue // Skip this file
		}

		// Construct the full path *safely*
		filePath := filepath.Join(uploadDir, baseFilename)

		// Delete the file
		err := os.Remove(filePath)
		if err != nil {
			// Check if it was because the file didn't exist (maybe deleted by another request?)
			if os.IsNotExist(err) {
				log.Printf("Attempted deletion of already non-existent file: %s", filePath)
				errorMessages = append(errorMessages, fmt.Sprintf("File not found: %s", baseFilename))
			} else {
				// Other deletion error
				log.Printf("Error deleting file %s: %v", filePath, err)
				errorMessages = append(errorMessages, fmt.Sprintf("Could not delete %s", baseFilename))
			}
		} else {
			// Successfully deleted
			log.Printf("Successfully deleted file: %s\n", baseFilename)
			deletedCount++
		}
	}

	// --- Prepare redirect messages ---
	var finalMessage, finalError string

	if deletedCount > 0 {
		finalMessage = fmt.Sprintf("Successfully deleted %d file(s).", deletedCount)
	}

	if len(errorMessages) > 0 {
		finalError = fmt.Sprintf("Errors occurred: %s", strings.Join(errorMessages, "; "))
		if finalMessage != "" { // If some succeeded and some failed
			finalMessage += " Some files could not be deleted."
		} else { // If only errors occurred
            finalMessage = "" // Clear any potential default success message
        }
	} else if deletedCount == 0 && len(filenamesToDelete) > 0 {
        // This case handles if all selected files resulted in errors (e.g., all were invalid names)
        finalError = "No files were deleted. See details above." // Error message already contains details
    }


	// Redirect back to index page with messages
	redirectURL := "/?"
	if finalMessage != "" {
		redirectURL += "message=" + url.QueryEscape(finalMessage)
	}
	if finalError != "" {
		if finalMessage != "" {
			redirectURL += "&" // Add separator if both messages exist
		}
		redirectURL += "error=" + url.QueryEscape(finalError)
	}
    if redirectURL == "/?" { // Handle case where somehow no message/error is set (e.g., empty selection bypassed initial check)
        redirectURL = "/"
    }

	http.Redirect(w, r, redirectURL, http.StatusSeeOther)
}


// deleteHandler handles single file deletion via POST request (Kept for potential direct API use, but UI uses multi-delete)
func deleteHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get filename from form value
	filename := r.FormValue("filename")
	if filename == "" {
		redirectURL := "/?error=" + url.QueryEscape("Filename missing for deletion")
		http.Redirect(w, r, redirectURL, http.StatusSeeOther)
		return
	}

	// *** Security Crucial: Sanitize filename ***
	baseFilename := filepath.Base(filename)
	if baseFilename == "." || baseFilename == ".." || baseFilename != filename {
		log.Printf("Attempted deletion with invalid filename: %s", filename)
		redirectURL := "/?error=" + url.QueryEscape("Invalid filename for deletion")
		http.Redirect(w, r, redirectURL, http.StatusSeeOther)
		return
	}

	// Construct the full path *safely*
	filePath := filepath.Join(uploadDir, baseFilename)

	// Check if file exists before attempting deletion (optional but good practice)
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		log.Printf("Attempted deletion of non-existent file: %s", filePath)
		redirectURL := "/?error=" + url.QueryEscape("File not found: "+baseFilename)
		http.Redirect(w, r, redirectURL, http.StatusSeeOther)
		return
	} else if err != nil {
        log.Printf("Error checking file status before delete: %v", err)
        redirectURL := "/?error=" + url.QueryEscape("Error checking file status for: "+baseFilename)
		http.Redirect(w, r, redirectURL, http.StatusSeeOther)
        return
    }

	// Delete the file
	err := os.Remove(filePath)
	if err != nil {
		log.Printf("Error deleting file %s: %v", filePath, err)
		redirectURL := "/?error=" + url.QueryEscape("Could not delete file: "+baseFilename)
		http.Redirect(w, r, redirectURL, http.StatusSeeOther)
		return
	}

	log.Printf("Successfully deleted file: %s\n", baseFilename)
	redirectURL := "/?message=" + url.QueryEscape("File '"+baseFilename+"' deleted successfully")
	http.Redirect(w, r, redirectURL, http.StatusSeeOther)
}

// listFiles reads the upload directory and returns a slice of filenames
func listFiles(dir string) ([]string, error) {
	var files []string
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			files = append(files, entry.Name())
		}
	}
	return files, nil
}