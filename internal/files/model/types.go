package model

import "time"

const (
	TargetTypeLocal = "local"
	ArchiveZip      = "zip"
	ArchiveTarGz    = "tar.gz"
)

type RootInfo struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

type RootsResponse struct {
	Enabled       bool       `json:"enabled"`
	AllowFullDisk bool       `json:"allow_full_disk"`
	OS            string     `json:"os"`
	PathSeparator string     `json:"path_separator"`
	Roots         []RootInfo `json:"roots"`
}

type TargetRequest struct {
	TargetType string `json:"target_type" form:"target_type"`
	NodeID     int    `json:"node_id" form:"node_id"`
}

type ListRequest struct {
	TargetRequest
	Path       string `json:"path" form:"path" binding:"required"`
	Page       int    `json:"page" form:"page"`
	Size       int    `json:"size" form:"size"`
	Search     string `json:"search" form:"search"`
	ShowHidden bool   `json:"show_hidden" form:"show_hidden"`
	SortBy     string `json:"sort_by" form:"sort_by"`
	SortOrder  string `json:"sort_order" form:"sort_order"`
}

type TreeRequest struct {
	TargetRequest
	Path  string `json:"path" binding:"required"`
	Depth int    `json:"depth"`
}

type ContentRequest struct {
	TargetRequest
	Path string `json:"path" binding:"required"`
}

type SaveRequest struct {
	TargetRequest
	Path    string `json:"path" binding:"required"`
	Content string `json:"content"`
}

type CreateRequest struct {
	TargetRequest
	Path  string `json:"path" binding:"required"`
	IsDir bool   `json:"is_dir"`
}

type RenameRequest struct {
	TargetRequest
	Path    string `json:"path" binding:"required"`
	NewName string `json:"new_name" binding:"required"`
}

type DeleteRequest struct {
	TargetRequest
	Path string `json:"path" binding:"required"`
}

type MoveRequest struct {
	TargetRequest
	Paths      []string `json:"paths" binding:"required"`
	TargetPath string   `json:"target_path" binding:"required"`
	Operation  string   `json:"operation" binding:"required"`
	Overwrite  bool     `json:"overwrite"`
}

type ChmodRequest struct {
	TargetRequest
	Path string `json:"path" binding:"required"`
	Mode uint32 `json:"mode" binding:"required"`
}

type ChownRequest struct {
	TargetRequest
	Path  string `json:"path" binding:"required"`
	User  string `json:"user" binding:"required"`
	Group string `json:"group" binding:"required"`
}

type CompressRequest struct {
	TargetRequest
	Paths      []string `json:"paths" binding:"required"`
	TargetPath string   `json:"target_path" binding:"required"`
	Name       string   `json:"name" binding:"required"`
	Type       string   `json:"type" binding:"required"`
	Overwrite  bool     `json:"overwrite"`
}

type DecompressRequest struct {
	TargetRequest
	Path       string `json:"path" binding:"required"`
	TargetPath string `json:"target_path" binding:"required"`
	Type       string `json:"type" binding:"required"`
	Overwrite  bool   `json:"overwrite"`
}

type FileInfo struct {
	Name        string    `json:"name"`
	Path        string    `json:"path"`
	Extension   string    `json:"extension"`
	Size        int64     `json:"size"`
	IsDir       bool      `json:"is_dir"`
	IsSymlink   bool      `json:"is_symlink"`
	IsHidden    bool      `json:"is_hidden"`
	Mode        string    `json:"mode"`
	User        string    `json:"user"`
	Group       string    `json:"group"`
	ModTime     time.Time `json:"mod_time"`
	MimeType    string    `json:"mime_type"`
	Editable    bool      `json:"editable"`
	Previewable bool      `json:"previewable"`
	Content     string    `json:"content,omitempty"`
}

type ListResponse struct {
	Path  string     `json:"path"`
	Items []FileInfo `json:"items"`
	Total int        `json:"total"`
}

type TreeNode struct {
	Name     string     `json:"name"`
	Path     string     `json:"path"`
	IsDir    bool       `json:"is_dir"`
	Children []TreeNode `json:"children,omitempty"`
}
