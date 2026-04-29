package service

import "errors"

var (
	ErrFilePathDenied           = errors.New("file path denied")
	ErrFileNotFound             = errors.New("file not found")
	ErrFileAlreadyExists        = errors.New("file already exists")
	ErrFileTooLarge             = errors.New("file too large")
	ErrFileBinaryUnsupported    = errors.New("binary file unsupported")
	ErrFileOperationUnsupported = errors.New("file operation unsupported")
	ErrFileInvalidName          = errors.New("invalid file name")
	ErrFileArchiveUnsupported   = errors.New("archive type unsupported")
)
