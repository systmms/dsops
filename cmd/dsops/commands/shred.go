package commands

import (
	"crypto/rand"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/systmms/dsops/internal/config"
	dserrors "github.com/systmms/dsops/internal/errors"
)

func NewShredCommand(cfg *config.Config) *cobra.Command {
	var (
		force       bool
		verbose     bool
		passes      int
		recursive   bool
	)

	cmd := &cobra.Command{
		Use:   "shred [paths...]",
		Short: "Securely delete files",
		Long: `Securely delete secret files to prevent recovery.

This command overwrites files with random data multiple times before deletion
to make recovery more difficult. Use with caution - this operation is irreversible.

Examples:
  dsops shred secret.env              # Delete a single file
  dsops shred secret.env db.json      # Delete multiple files  
  dsops shred --recursive secrets/    # Delete directory recursively
  dsops shred --passes 3 --verbose secret.env  # Custom overwrite passes

Security Note: 
Modern SSDs with wear leveling may still retain data. For maximum security,
use full disk encryption and proper key management.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return dserrors.UserError{
					Message:    "No files specified",
					Suggestion: "Provide one or more file paths to shred",
				}
			}

			return shredFiles(args, force, verbose, passes, recursive)
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Force deletion without confirmation")
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Show detailed progress")
	cmd.Flags().IntVarP(&passes, "passes", "n", 3, "Number of overwrite passes (default: 3)")
	cmd.Flags().BoolVarP(&recursive, "recursive", "r", false, "Recursively shred directories")

	return cmd
}

func shredFiles(paths []string, force, verbose bool, passes int, recursive bool) error {
	if passes < 1 || passes > 10 {
		return dserrors.UserError{
			Message:    "Invalid number of passes",
			Suggestion: "Passes must be between 1 and 10",
		}
	}

	var filesToShred []string

	// Collect all files to shred
	for _, path := range paths {
		files, err := collectFiles(path, recursive)
		if err != nil {
			return err
		}
		filesToShred = append(filesToShred, files...)
	}

	if len(filesToShred) == 0 {
		fmt.Println("No files found to shred")
		return nil
	}

	// Show what will be shredded
	fmt.Printf("Files to be securely deleted (%d passes):\n", passes)
	for _, file := range filesToShred {
		fmt.Printf("  %s\n", file)
	}
	fmt.Println()

	// Confirmation
	if !force {
		fmt.Print("⚠️  This operation is IRREVERSIBLE. Continue? (y/N): ")
		var response string
		_, _ = fmt.Scanln(&response)
		if strings.ToLower(response) != "y" && strings.ToLower(response) != "yes" {
			fmt.Println("Operation cancelled")
			return nil
		}
	}

	// Shred files
	for _, file := range filesToShred {
		if verbose {
			fmt.Printf("Shredding: %s\n", file)
		}
		if err := shredFile(file, passes, verbose); err != nil {
			fmt.Printf("Error shredding %s: %v\n", file, err)
		} else if verbose {
			fmt.Printf("✅ Shredded: %s\n", file)
		}
	}

	fmt.Printf("✅ Securely deleted %d files\n", len(filesToShred))
	return nil
}

func collectFiles(path string, recursive bool) ([]string, error) {
	var files []string

	info, err := os.Stat(path)
	if err != nil {
		return nil, dserrors.UserError{
			Message:    fmt.Sprintf("Cannot access path: %s", path),
			Details:    err.Error(),
			Suggestion: "Check that the file or directory exists and is accessible",
		}
	}

	if !info.IsDir() {
		// It's a file
		return []string{path}, nil
	}

	// It's a directory
	if !recursive {
		return nil, dserrors.UserError{
			Message:    fmt.Sprintf("Path is a directory: %s", path),
			Suggestion: "Use --recursive to shred directories",
		}
	}

	err = filepath.Walk(path, func(walkPath string, walkInfo os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !walkInfo.IsDir() {
			files = append(files, walkPath)
		}
		return nil
	})

	if err != nil {
		return nil, dserrors.UserError{
			Message:    fmt.Sprintf("Error walking directory: %s", path),
			Details:    err.Error(),
		}
	}

	return files, nil
}

func shredFile(path string, passes int, verbose bool) error {
	// Get file info
	info, err := os.Stat(path)
	if err != nil {
		return err
	}

	size := info.Size()
	if size == 0 {
		// Empty file, just delete it
		return os.Remove(path)
	}

	// Open file for writing
	file, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

	// Perform overwrite passes
	for pass := 1; pass <= passes; pass++ {
		if verbose {
			fmt.Printf("  Pass %d/%d...\n", pass, passes)
		}

		// Seek to beginning
		if _, err := file.Seek(0, 0); err != nil {
			return err
		}

		// Overwrite with random data
		if err := overwriteWithRandom(file, size); err != nil {
			return err
		}

		// Sync to ensure data is written
		if err := file.Sync(); err != nil {
			return err
		}
	}

	_ = file.Close()

	// Finally, delete the file
	return os.Remove(path)
}

func overwriteWithRandom(w io.Writer, size int64) error {
	const bufSize = 64 * 1024 // 64KB buffer

	buf := make([]byte, bufSize)
	remaining := size

	for remaining > 0 {
		writeSize := bufSize
		if remaining < int64(bufSize) {
			writeSize = int(remaining)
		}

		// Generate random data
		if _, err := rand.Read(buf[:writeSize]); err != nil {
			return err
		}

		// Write random data
		if _, err := w.Write(buf[:writeSize]); err != nil {
			return err
		}

		remaining -= int64(writeSize)
	}

	return nil
}