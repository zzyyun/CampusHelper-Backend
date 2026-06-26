package model

import "testing"

func TestFileCategories(t *testing.T) {
	categories := map[FileCategory]string{
		FileCategoryAvatar: "avatar",
		FileCategoryPost:   "post",
		FileCategoryTask:   "task",
		FileCategoryOther:  "other",
	}
	for k, v := range categories {
		if string(k) != v {
			t.Errorf("FileCategory(%v) != %q", k, v)
		}
	}
}

func TestFile_TableName(t *testing.T) {
	var f File
	if f.TableName() != "files" {
		t.Errorf("TableName 应为 files，实际 %s", f.TableName())
	}
}