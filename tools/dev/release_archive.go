package main

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

// validateArchive 读取真实归档，核对用户解压后需要的根目录文件。
func validateArchive(path, binary string) error {
	required := map[string]bool{binary: false, "README.md": false, "config.example.yaml": false}
	accept := func(name string, regular, nonempty bool) error {
		found, needed := required[name]
		if !needed {
			return nil
		}
		if found || !regular || !nonempty {
			return fmt.Errorf("归档必要文件重复、为空或非常规：%s", name)
		}
		required[name] = true
		return nil
	}
	if strings.HasSuffix(path, ".zip") {
		archive, err := zip.OpenReader(path)
		if err != nil {
			return fmt.Errorf("读取 zip 归档失败：%w", err)
		}
		defer func() { _ = archive.Close() }()
		for _, file := range archive.File {
			if err := accept(file.Name, file.Mode().IsRegular(), file.UncompressedSize64 > 0); err != nil {
				return err
			}
			reader, err := file.Open()
			if err != nil {
				return err
			}
			_, readErr := io.Copy(io.Discard, reader)
			closeErr := reader.Close()
			if err := errors.Join(readErr, closeErr); err != nil {
				return fmt.Errorf("读取归档文件失败：%w", err)
			}
		}
	} else if strings.HasSuffix(path, ".tar.gz") {
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer func() { _ = file.Close() }()
		compressed, err := gzip.NewReader(file)
		if err != nil {
			return fmt.Errorf("读取 gzip 归档失败：%w", err)
		}
		defer func() { _ = compressed.Close() }()
		archive := tar.NewReader(compressed)
		for {
			header, err := archive.Next()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				return fmt.Errorf("读取 tar 归档失败：%w", err)
			}
			regular := header.Typeflag == tar.TypeReg
			if err := accept(header.Name, regular, header.Size > 0); err != nil {
				return err
			}
			if _, err := io.Copy(io.Discard, archive); err != nil {
				return err
			}
		}
		if _, err := io.Copy(io.Discard, compressed); err != nil {
			return fmt.Errorf("归档压缩数据损坏：%w", err)
		}
	} else {
		return fmt.Errorf("不支持的发布归档格式：%s", path)
	}
	for name, found := range required {
		if !found {
			return fmt.Errorf("归档缺少根目录必要文件：%s", name)
		}
	}
	return nil
}
