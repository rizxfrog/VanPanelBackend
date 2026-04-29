package fs

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func CompressZip(paths []string, dst string) error {
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	zw := zip.NewWriter(out)
	defer zw.Close()

	for _, src := range paths {
		if err := filepath.Walk(src, func(pathValue string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				return nil
			}
			header, err := zip.FileInfoHeader(info)
			if err != nil {
				return err
			}
			header.Name = filepath.ToSlash(filepath.Base(src))
			if pathValue != src {
				rel, _ := filepath.Rel(filepath.Dir(src), pathValue)
				header.Name = filepath.ToSlash(rel)
			}
			writer, err := zw.CreateHeader(header)
			if err != nil {
				return err
			}
			in, err := os.Open(pathValue)
			if err != nil {
				return err
			}
			defer in.Close()
			_, err = io.Copy(writer, in)
			return err
		}); err != nil {
			return err
		}
	}
	return nil
}

func CompressTarGz(paths []string, dst string) error {
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	gw := gzip.NewWriter(out)
	defer gw.Close()
	tw := tar.NewWriter(gw)
	defer tw.Close()

	for _, src := range paths {
		if err := filepath.Walk(src, func(pathValue string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			header, err := tar.FileInfoHeader(info, "")
			if err != nil {
				return err
			}
			header.Name = filepath.ToSlash(filepath.Base(src))
			if pathValue != src {
				rel, _ := filepath.Rel(filepath.Dir(src), pathValue)
				header.Name = filepath.ToSlash(rel)
			}
			if err := tw.WriteHeader(header); err != nil {
				return err
			}
			if info.IsDir() {
				return nil
			}
			in, err := os.Open(pathValue)
			if err != nil {
				return err
			}
			defer in.Close()
			_, err = io.Copy(tw, in)
			return err
		}); err != nil {
			return err
		}
	}
	return nil
}

func ExtractZip(src, dst string, overwrite bool) error {
	reader, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer reader.Close()

	for _, file := range reader.File {
		target, err := safeJoin(dst, file.Name)
		if err != nil {
			return err
		}
		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(target, file.Mode()); err != nil {
				return err
			}
			continue
		}
		if _, err := os.Stat(target); err == nil && !overwrite {
			return os.ErrExist
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		in, err := file.Open()
		if err != nil {
			return err
		}
		out, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, file.Mode())
		if err != nil {
			in.Close()
			return err
		}
		_, copyErr := io.Copy(out, in)
		closeErr := out.Close()
		in.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return nil
}

func ExtractTarGz(src, dst string, overwrite bool) error {
	file, err := os.Open(src)
	if err != nil {
		return err
	}
	defer file.Close()

	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gzipReader.Close()
	tarReader := tar.NewReader(gzipReader)

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		target, err := safeJoin(dst, header.Name)
		if err != nil {
			return err
		}
		if header.FileInfo().IsDir() {
			if err := os.MkdirAll(target, header.FileInfo().Mode()); err != nil {
				return err
			}
			continue
		}
		if _, err := os.Stat(target); err == nil && !overwrite {
			return os.ErrExist
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		out, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, header.FileInfo().Mode())
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(out, tarReader)
		closeErr := out.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
	}
}

func safeJoin(baseDir, name string) (string, error) {
	clean := filepath.Clean(filepath.Join(baseDir, name))
	if clean != baseDir && !strings.HasPrefix(clean, baseDir+string(filepath.Separator)) {
		return "", os.ErrPermission
	}
	return clean, nil
}
