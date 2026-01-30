package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tpott/subtitler/backend/crypto"
	"github.com/tpott/subtitler/backend/db"
)

func usage() {
	fmt.Fprintf(os.Stderr, `Usage: %s <command> [options]

Commands:
  status     Show current key rotation status
  rotate     Generate a new encryption key and make it current
  reencrypt  Re-encrypt files using old keys with the current key

Options:
`, os.Args[0])
	flag.PrintDefaults()
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	// Get env vars
	keysDir := os.Getenv("KEYS_DIR")
	if keysDir == "" {
		keysDir = "data/keys"
	}
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "data/subtitler.db"
	}

	command := os.Args[1]

	// Remove the command from os.Args so flag.Parse works correctly
	os.Args = append([]string{os.Args[0]}, os.Args[2:]...)

	switch command {
	case "status":
		cmdStatus(keysDir, dbPath)
	case "rotate":
		cmdRotate(keysDir, dbPath)
	case "reencrypt":
		cmdReencrypt(keysDir, dbPath)
	case "help", "-h", "--help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", command)
		usage()
		os.Exit(1)
	}
}

func cmdStatus(keysDir, dbPath string) {
	m := crypto.NewMultiKeyEncryptor(keysDir)
	if err := m.LoadOrInitialize(); err != nil {
		fmt.Fprintf(os.Stderr, "Error loading keys: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Keys Directory: %s\n", keysDir)
	fmt.Printf("Current Version: %d\n", m.GetCurrentVersion())
	fmt.Printf("Available Versions: %v\n", m.GetVersions())

	// Show file counts per version
	database, err := db.Open(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "\nWarning: Could not open database: %v\n", err)
		return
	}
	defer database.Close()

	counts, err := database.CountVideosByKeyVersion()
	if err != nil {
		fmt.Fprintf(os.Stderr, "\nWarning: Could not count videos: %v\n", err)
		return
	}

	fmt.Printf("\nFiles per key version:\n")
	if len(counts) == 0 {
		fmt.Println("  (no videos in database)")
	} else {
		totalFiles := 0
		totalOldFiles := 0
		currentVersion := m.GetCurrentVersion()
		for version, count := range counts {
			marker := ""
			if version == currentVersion {
				marker = " (current)"
			} else {
				totalOldFiles += count
			}
			fmt.Printf("  Version %d: %d files%s\n", version, count, marker)
			totalFiles += count
		}
		if totalOldFiles > 0 {
			fmt.Printf("\n%d files need re-encryption to version %d\n", totalOldFiles, currentVersion)
		} else {
			fmt.Println("\nAll files are encrypted with the current key.")
		}
	}
}

func cmdRotate(keysDir, dbPath string) {
	m := crypto.NewMultiKeyEncryptor(keysDir)
	if err := m.LoadOrInitialize(); err != nil {
		fmt.Fprintf(os.Stderr, "Error loading keys: %v\n", err)
		os.Exit(1)
	}

	oldVersion := m.GetCurrentVersion()
	newVersion, err := m.Rotate()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error rotating key: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Key rotated successfully!\n")
	fmt.Printf("Old version: %d\n", oldVersion)
	fmt.Printf("New version: %d\n", newVersion)
	fmt.Printf("New key file: %s/key_v%d.age\n", keysDir, newVersion)

	// Show what needs re-encryption
	database, err := db.Open(dbPath)
	if err != nil {
		fmt.Printf("\nNote: Could not open database to check files: %v\n", err)
		return
	}
	defer database.Close()

	counts, err := database.CountVideosByKeyVersion()
	if err != nil {
		fmt.Printf("\nNote: Could not count videos: %v\n", err)
		return
	}

	totalOldFiles := 0
	for version, count := range counts {
		if version < newVersion {
			totalOldFiles += count
		}
	}

	if totalOldFiles > 0 {
		fmt.Printf("\n%d files are encrypted with older keys.\n", totalOldFiles)
		fmt.Println("Run 'rotate-keys reencrypt' to re-encrypt them with the new key.")
	}
}

func cmdReencrypt(keysDir, dbPath string) {
	batchSize := flag.Int("batch", 100, "Number of files to re-encrypt per batch")
	dryRun := flag.Bool("dry-run", false, "Show what would be done without making changes")
	flag.Parse()

	m := crypto.NewMultiKeyEncryptor(keysDir)
	if err := m.LoadOrInitialize(); err != nil {
		fmt.Fprintf(os.Stderr, "Error loading keys: %v\n", err)
		os.Exit(1)
	}

	database, err := db.Open(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening database: %v\n", err)
		os.Exit(1)
	}
	defer database.Close()

	currentVersion := m.GetCurrentVersion()

	// Get videos with old key versions
	videos, err := database.GetVideosWithOldKeyVersion(currentVersion, *batchSize)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting videos: %v\n", err)
		os.Exit(1)
	}

	if len(videos) == 0 {
		fmt.Println("All files are already encrypted with the current key version.")
		return
	}

	fmt.Printf("Found %d files to re-encrypt (batch size: %d)\n", len(videos), *batchSize)
	if *dryRun {
		fmt.Println("\nDry run mode - no changes will be made.")
	}

	successCount := 0
	errorCount := 0

	for _, video := range videos {
		if *dryRun {
			fmt.Printf("Would re-encrypt: %s (v%d -> v%d)\n", video.ID, video.KeyVersion, currentVersion)
			successCount++
			continue
		}

		// Skip if file path doesn't end with .age (unencrypted file)
		if !strings.HasSuffix(video.FilePath, ".age") {
			fmt.Printf("Skipping %s: not an encrypted file\n", video.ID)
			continue
		}

		// Check if file exists
		if _, err := os.Stat(video.FilePath); os.IsNotExist(err) {
			fmt.Printf("Error %s: file not found at %s\n", video.ID, video.FilePath)
			errorCount++
			continue
		}

		// Re-encrypt the video file
		newPath, newVersion, err := m.ReencryptFile(video.FilePath, video.KeyVersion)
		if err != nil {
			fmt.Printf("Error re-encrypting %s: %v\n", video.ID, err)
			errorCount++
			continue
		}

		// Update database
		if err := database.UpdateVideoKeyVersion(video.ID, newVersion, newPath); err != nil {
			fmt.Printf("Error updating database for %s: %v\n", video.ID, err)
			// Try to clean up new file since DB update failed
			if err := os.Remove(newPath); err != nil && !os.IsNotExist(err) {
				fmt.Fprintf(os.Stderr, "Warning: could not clean up new file %s: %v\n", newPath, err)
			}
			errorCount++
			continue
		}

		// Delete old encrypted file
		if err := os.Remove(video.FilePath); err != nil {
			fmt.Printf("Warning: could not delete old file for %s: %v\n", video.ID, err)
		}

		// Re-encrypt thumbnail if exists
		if video.ThumbnailPath != nil && strings.HasSuffix(*video.ThumbnailPath, ".age") {
			if _, err := os.Stat(*video.ThumbnailPath); err == nil {
				newThumbPath, _, err := m.ReencryptFile(*video.ThumbnailPath, video.KeyVersion)
				if err != nil {
					fmt.Printf("Warning: could not re-encrypt thumbnail for %s: %v\n", video.ID, err)
				} else {
					if err := database.UpdateVideoThumbnailKeyVersion(video.ID, newThumbPath); err != nil {
						fmt.Printf("Warning: could not update thumbnail path for %s: %v\n", video.ID, err)
						if err := os.Remove(newThumbPath); err != nil && !os.IsNotExist(err) {
							fmt.Fprintf(os.Stderr, "Warning: could not clean up new thumbnail %s: %v\n", newThumbPath, err)
						}
					} else {
						if err := os.Remove(*video.ThumbnailPath); err != nil && !os.IsNotExist(err) {
							fmt.Fprintf(os.Stderr, "Warning: could not delete old thumbnail %s: %v\n", *video.ThumbnailPath, err)
						}
					}
				}
			}
		}

		fmt.Printf("Re-encrypted %s: v%d -> v%d\n", video.ID, video.KeyVersion, newVersion)
		successCount++
	}

	fmt.Printf("\nCompleted: %d successful, %d errors\n", successCount, errorCount)

	// Check if more files need processing
	remaining, err := database.GetVideosWithOldKeyVersion(currentVersion, 1)
	if err == nil && len(remaining) > 0 {
		counts, _ := database.CountVideosByKeyVersion()
		totalRemaining := 0
		for version, count := range counts {
			if version < currentVersion {
				totalRemaining += count
			}
		}
		fmt.Printf("\n%d more files need re-encryption. Run again to continue.\n", totalRemaining)
	}
}

func init() {
	// Check if running from backend directory
	if _, err := os.Stat("data"); os.IsNotExist(err) {
		// Try to find the data directory relative to executable
		exe, err := os.Executable()
		if err == nil {
			dir := filepath.Dir(exe)
			// Walk up looking for data directory
			for i := 0; i < 5; i++ {
				if _, err := os.Stat(filepath.Join(dir, "data")); err == nil {
					os.Chdir(dir)
					break
				}
				dir = filepath.Dir(dir)
			}
		}
	}
}
