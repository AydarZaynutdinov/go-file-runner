package main

import (
	"bytes"
	"fmt"
	"io/fs"
	"io/ioutil"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

const (
	datePattern = "20060102150405"
)

func main() {
	http.HandleFunc("/run", receiveFile)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "Hi there!!")
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
		log.Printf("Set default port %s\n", port)
	}
	log.Printf("Listening on port %s\n", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func receiveFile(w http.ResponseWriter, r *http.Request) {
	key, file, filInfo, err := getParams(r)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		message := fmt.Sprintf("Error during get params from request's body: %v", err)
		w.Write([]byte(message))
		return
	}

	fileName := getFileName(key)
	folderName, err := createFolder(key)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		message := fmt.Sprintf("Error during create folder: %s. err: %s", folderName, err)
		w.Write([]byte(message))
		return
	}

	err = saveFile(file, filInfo, fileName, folderName)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		message := fmt.Sprintf("Error during save file: %s", err)
		w.Write([]byte(message))
		return
	}

	defer func(fileName string, folderName string) {
		err := removeFile(fileName, folderName)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			message := fmt.Sprintf("Error during delete file from Google Cloud Storage: %s", err)
			w.Write([]byte(message))
			return
		}
	}(fileName, folderName)

	err = runFile(fileName, folderName, w)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		message := fmt.Sprintf("Error during run file: %s. err: %s", fileName, err)
		w.Write([]byte(message))
		return
	}
}

//Get params from request's body
func getParams(r *http.Request) (string, multipart.File, *multipart.FileHeader, error) {
	key := r.FormValue("key")
	file, fileInfo, err := r.FormFile("file")
	if err != nil {
		log.Printf("getParams: error during reading params from request's body: %v", err)
		return "", nil, nil, err
	}
	return key, file, fileInfo, nil
}

//Construct fileName
func getFileName(key string) string {
	return key + time.Now().Format(datePattern) + ".go"
}

//Create folder
func createFolder(key string) (string, error) {
	folderName := key + time.Now().Format(datePattern)
	err := os.Mkdir(folderName, fs.ModeDir)
	if err != nil {
		log.Printf("saveFile: error during create folder: %s", folderName)
		return "", err
	}
	return folderName, nil
}

//Save file
func saveFile(file multipart.File, fileInfo *multipart.FileHeader, fileName string, folderName string) error {
	fileData := make([]byte, fileInfo.Size)
	_, err := file.Read(fileData)
	if err != nil {
		log.Printf("saveFile: error during read file: %s", fileName)
		return err
	}

	err = ioutil.WriteFile(folderName+"/"+fileName, fileData, 0600)
	if err != nil {
		log.Printf("saveFile: error during save file: %s", fileName)
		return err
	}
	return nil
}

//Run file
func runFile(fileName string, folderName string, w http.ResponseWriter) error {
	runModTidy(fileName)

	return runCode(fileName, folderName, w)
}

func runModTidy(fileName string) {
	var modStdErr bytes.Buffer
	var modStdOut bytes.Buffer

	modCmd := exec.Command("go", "mod", "tidy")
	modCmd.Stderr = &modStdErr
	modCmd.Stdout = &modStdOut
	err := modCmd.Run()
	if err != nil {
		log.Printf("runFile: error during run `mode tidy` for file: %s", fileName)
	}
	if modStdErr.Len() != 0 {
		log.Println("mod tidy error:")
		log.Println(modStdErr.String())
	}
	if modStdOut.Len() != 0 {
		log.Println("mod tidy out:")
		log.Println(modStdOut.String())
	}
}

func runCode(fileName string, folderName string, w http.ResponseWriter) error {
	var stdErr bytes.Buffer
	var stdOut bytes.Buffer

	cmd := exec.Command("go", "run", filepath.Join(folderName, fileName))
	cmd.Stdout = &stdOut
	cmd.Stderr = &stdErr
	err := cmd.Run()
	if err != nil {
		log.Printf("runFile: error during run file: %s", fileName)
	}
	if stdErr.Len() != 0 {
		log.Println(stdErr.String())
		fmt.Fprint(w, stdErr.String())
		err = nil
	}
	if stdOut.Len() != 0 {
		log.Println(stdOut.String())
		fmt.Fprint(w, stdOut.String())
		err = nil
	}
	return err
}

//Remove file
func removeFile(fileName string, folderName string) error {
	err := os.Remove(filepath.Join(folderName, fileName))
	if err != nil {
		log.Printf("removeFile: error during remove file: %s", fileName)
		return err
	}

	err = os.Remove(folderName)
	if err != nil {
		log.Printf("removeFile: error during remove folder: %s", folderName)
		return err
	}
	return nil
}
