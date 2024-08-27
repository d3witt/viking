package archive

import (
	"archive/tar"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/d3witt/viking/sshexec"
)

func Tar(source string) (io.Reader, error) {
	pr, pw := io.Pipe()
	tw := tar.NewWriter(pw)

	go func() {
		defer func() {
			if err := tw.Close(); err != nil {
				fmt.Println("Error closing tar writer:", err)
			}
			pw.Close() // Close the pipe writer when done
		}()

		fi, err := os.Stat(source)
		if err != nil {
			pw.CloseWithError(err)
			return
		}

		if fi.IsDir() {
			err = filepath.Walk(source, func(filePath string, fi os.FileInfo, err error) error {
				if err != nil {
					return err
				}

				// Construct the header
				relPath, err := filepath.Rel(source, filePath)
				if err != nil {
					return err
				}
				header, err := tar.FileInfoHeader(fi, "")
				if err != nil {
					return err
				}

				// Use relative path to avoid including the entire source directory structure
				header.Name = relPath

				// Write the header
				if err := tw.WriteHeader(header); err != nil {
					return err
				}

				// If it's a regular file, write its content to the tar writer
				if fi.Mode().IsRegular() {
					file, err := os.Open(filePath)
					if err != nil {
						return err
					}
					defer file.Close()

					if _, err := io.Copy(tw, file); err != nil {
						return err
					}
				}

				return nil
			})
		} else {
			// Handle the case where source is a single file
			header, err := tar.FileInfoHeader(fi, "")
			if err != nil {
				pw.CloseWithError(err)
				return
			}

			// Only set the base name for a single file
			header.Name = filepath.Base(source)

			// Write the header
			if err := tw.WriteHeader(header); err != nil {
				pw.CloseWithError(err)
				return
			}

			// Write the file content
			file, err := os.Open(source)
			if err != nil {
				pw.CloseWithError(err)
				return
			}
			defer file.Close()

			if _, err := io.Copy(tw, file); err != nil {
				pw.CloseWithError(err)
				return
			}
		}

		if err != nil {
			pw.CloseWithError(err) // Close the pipe with an error if it occurs
		}
	}()

	return pr, nil
}

func Untar(r io.Reader, dest string) error {
	tr := tar.NewReader(r)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break // End of tar archive
		}
		if err != nil {
			return err
		}

		// Create the file or directory
		target := filepath.Join(dest, header.Name)
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, os.FileMode(header.Mode)); err != nil {
				return err
			}
		case tar.TypeReg:
			dir := filepath.Dir(target)
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return err
			}
			file, err := os.OpenFile(target, os.O_RDWR|os.O_CREATE|os.O_TRUNC, os.FileMode(header.Mode))
			if err != nil {
				return err
			}
			defer file.Close()
			if _, err := io.Copy(file, tr); err != nil {
				return err
			}
		default:
			continue
		}
	}

	return nil
}

// TarRemote creates a tar archive for the given file/directory on the remote server.
func TarRemote(exec sshexec.Executor, source string) (io.Reader, error) {
	outPipe, inPipe := io.Pipe()

	go func() {
		defer inPipe.Close()
		cmd := sshexec.Command(exec, "tar", "-cf", "-", source, ".")
		cmd.Stdout = inPipe
		if err := cmd.Run(); err != nil {
			inPipe.CloseWithError(err)
		}
	}()

	return outPipe, nil
}

func UntarRemote(exec sshexec.Executor, dest string, in io.Reader) error {
	folderPath := filepath.Dir(dest)

	// Ensure the destination directory exists
	cmd := sshexec.Command(exec, "mkdir", "-p", folderPath)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Untar the contents to the destination directory, replacing existing files
	cmd = sshexec.Command(exec, "tar", "--overwrite", "-xf", "-", "-C", folderPath)
	cmd.Stdin = in

	return cmd.Run()
}
