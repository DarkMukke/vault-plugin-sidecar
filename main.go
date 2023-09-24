package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	vault "github.com/hashicorp/vault/api"
	auth "github.com/hashicorp/vault/api/auth/kubernetes"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"gopkg.in/yaml.v3"
	"io"
	"log"
	"os"
)

type Config struct {
	S3      S3Config                `yaml:"s3"`
	Vault   VaultConfig             `yaml:"vault"`
	Plugins map[string]PluginConfig `yaml:"plugins"`
}

type S3Config struct {
	Endpoint string `yaml:"endpoint"`
	Bucket   string `yaml:"bucket"`
	Token    string `yaml:"token"`
	Key      string `yaml:"key"`
}

type VaultConfig struct {
	PluginDir string `yaml:"plugin-dir"`
}

type PluginConfig struct {
	Version string            `yaml:"version"`
	Type    string            `yaml:"type"`
	Arch    map[string]string `yaml:"arch"`
}

func readConfig(filePath string) (*Config, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var config Config
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return nil, err
	}

	return &config, nil
}

func downloadFile(client *minio.Client, bucket, objectName, filePath string) error {
	err := client.FGetObject(context.Background(), bucket, objectName, filePath, minio.GetObjectOptions{})
	return err
}

func fileSHA256(filePath string) (string, error) {
	// Step 1: Open the file
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	// Step 2: Create a new Hasher
	hasher := sha256.New()

	// Step 3: Copy the file content to the Hasher
	_, err = io.Copy(hasher, file)
	if err != nil {
		return "", err
	}

	// Step 4: Get the hash
	hash := hasher.Sum(nil)

	// Return the hex representation of the hash
	return hex.EncodeToString(hash), nil
}

func main() {

	// 1. Read Environment variables and mounts
	configFile := os.Getenv("CONFIG")
	if configFile == "" {
		// Handle the error case where ARCH is not set
		log.Fatal("Config Map required")
	}

	vaultAddr := os.Getenv("VAULT_ADDR")
	if vaultAddr == "" {
		// Handle the error case where ARCH is not set
		log.Fatal("Vault Address required")
	}
	vaultServiceAccountName := os.Getenv("SA_NAME")
	if vaultServiceAccountName == "" {
		vaultServiceAccountName = "vault-plugin-sidecar"
	}

	vaultConfig := &vault.Config{
		Address: vaultAddr, // Replace with your Vault address
	}

	vaultClient, err := vault.NewClient(vaultConfig)
	if err != nil {
		log.Fatalf("Failed to create Vault client: %v", err)
	}
	k8sAuth, err := auth.NewKubernetesAuth(vaultServiceAccountName)
	if err != nil {
		log.Fatalf("unable to initialize Kubernetes auth method: %v", err)
	}
	authInfo, err := vaultClient.Auth().Login(context.Background(), k8sAuth)
	if err != nil {
		log.Fatalf("unable to log in with Kubernetes auth: %v", err)
	}
	if authInfo == nil {
		log.Fatal("no auth info was returned after login")
	}

	config, err := readConfig(configFile)
	if err != nil {
		log.Fatalf("Error reading config: %v", err)
	}

	// 2. Initialize MinIO Client
	s3Client, err := minio.New(config.S3.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(config.S3.Token, config.S3.Key, ""),
		Secure: true, // Set to false if not using TLS
	})
	if err != nil {
		log.Fatalf("Error initializing minio client: %v", err)
	}

	// 3. Loop through plugins and download
	systemArchitecture := os.Getenv("ARCH")
	if systemArchitecture == "" {
		systemArchitecture = "amd64" // Replace with your actual default value
	}
	for pluginName, pluginConfig := range config.Plugins {
		for arch, filename := range pluginConfig.Arch {
			log.Printf("Processing plugin: %s, version: %s, type: %s, arch: %s, filename: %s",
				pluginName, pluginConfig.Version, pluginConfig.Type, arch, filename)
			// Check if architecture matches
			if arch == systemArchitecture {
				// Construct file paths and download
				destPath := fmt.Sprintf("%s/%s", config.Vault.PluginDir, filename)
				err := downloadFile(s3Client, config.S3.Bucket, filename, destPath)
				if err != nil {
					log.Printf("Error downloading file: %v", err)
					continue
				}

				// 4. register the plugin with vault after downloading
				pluginSha, err := fileSHA256(destPath)
				if err != nil {
					log.Printf("Could not calculate sha256 hash for %v: %v", destPath, err)
					continue
				}
				parsedPluginType, err := vault.ParsePluginType(pluginConfig.Type)
				if err != nil {
					log.Printf("Unknow plugin type %v for %v: %v", pluginConfig.Type, filename, err)
					continue
				}
				vaultPlugin := vault.RegisterPluginInput{
					Name:    filename,
					Type:    parsedPluginType,
					SHA256:  pluginSha,
					Version: pluginConfig.Version,
				}

				err = vaultClient.Sys().RegisterPlugin(&vaultPlugin)
				if err != nil {
					log.Printf("Could not register plugin for %v: %v", filename, err)
					continue
				}
			}
		}

	}
}
