package main

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sync"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

const (
	bucketName = "hpecty"
	region     = "ap-south-1"
)

var (
	awsSession *session.Session
	s3Client   *s3.S3
	mu         sync.Mutex
)

func init() {
	var err error
	awsSession, err = session.NewSession(&aws.Config{
		Region: aws.String(region)},
	)
	if err != nil {
		log.Fatal("Unable to create AWS session: ", err)
	}
	s3Client = s3.New(awsSession)
}

func listAllFilesInBucket(bucket string) ([]string, error) {
	var files []string
	err := s3Client.ListObjectsV2Pages(&s3.ListObjectsV2Input{
		Bucket: aws.String(bucket),
	}, func(page *s3.ListObjectsV2Output, lastPage bool) bool {
		for _, item := range page.Contents {
			files = append(files, *item.Key)
		}
		return true
	})
	return files, err
}

func downloadFileFromS3(bucket, key string) ([]byte, error) {
	result, err := s3Client.GetObject(&s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, err
	}
	defer result.Body.Close()

	buf := new(bytes.Buffer)
	_, err = io.Copy(buf, result.Body)
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func zipFiles(files map[string][]byte) ([]byte, error) {
	buf := new(bytes.Buffer)
	zipWriter := zip.NewWriter(buf)
	for key, data := range files {
		f, err := zipWriter.Create(key)
		if err != nil {
			return nil, err
		}
		_, err = f.Write(data)
		if err != nil {
			return nil, err
		}
	}
	if err := zipWriter.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func createISO(fileContents map[string][]byte) ([]byte, error) {
    log.Println("ISO creation initiated")

    tempDir, err := ioutil.TempDir("", "s3files")
    if err != nil {
        return nil, err
    }
    defer os.RemoveAll(tempDir)

    for name, content := range fileContents {
        filePath := filepath.Join(tempDir, name)
        if err := os.MkdirAll(filepath.Dir(filePath), os.ModePerm); err != nil {
            return nil, err
        }
        if err := ioutil.WriteFile(filePath, content, os.ModePerm); err != nil {
            return nil, err
        }
    }

    isoFile, err := ioutil.TempFile("", "output.iso")
    if err != nil {
        return nil, err
    }
    isoFilePath := isoFile.Name()
    isoFile.Close()

    cmd := exec.Command("genisoimage", "-o", isoFilePath, "-R", "-J", tempDir)
    output, err := cmd.CombinedOutput()
    if err != nil {
        return nil, fmt.Errorf("genisoimage failed: %v, output: %s", err, output)
    }

    isoData, err := ioutil.ReadFile(isoFilePath)
    if err != nil {
        return nil, err
    }
    os.Remove(isoFilePath)

    log.Println("ISO creation completed")

    return isoData, nil
}

func handler(w http.ResponseWriter, r *http.Request) {
	// Set CORS headers
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	files, err := listAllFilesInBucket(bucketName)
	if err != nil {
		http.Error(w, "Error listing files: "+err.Error(), http.StatusInternalServerError)
		return
	}

	var wg sync.WaitGroup
	fileContents := make(map[string][]byte)

	for _, key := range files {
		wg.Add(1)
		go func(key string) {
			defer wg.Done()
			data, err := downloadFileFromS3(bucketName, key)
			if err != nil {
				log.Printf("Error downloading file %s: %v", key, err)
				return
			}
			mu.Lock()
			fileContents[key] = data
			mu.Unlock()
		}(key)
	}

	wg.Wait()

	if len(fileContents) == 0 {
		http.Error(w, "No files to process", http.StatusInternalServerError)
		return
	}

	isoData, err := createISO(fileContents)
	if err != nil {
		http.Error(w, "Error creating ISO: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Disposition", "attachment; filename=files.iso")
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Cache-Control", "no-store")
	w.Write(isoData)
}

func main() {
	http.HandleFunc("/download", handler)
	log.Println("Backend server started at :8081")
	if err := http.ListenAndServe(":8081", nil); err != nil {
		log.Fatal(err)
	}
}

