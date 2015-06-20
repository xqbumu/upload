// Copyright 2015 by caixw, All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

package upload

import (
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// 创建文件的默认权限，比如Upload.dir若不存在，会使用此权限创建目录。
const defaultMode os.FileMode = 0660

// Upload用于处理文件上传
type Upload struct {
	dir     string   // 上传文件保存的路径根目录
	maxSize int64    // 允许的最大文件大小，以byte为单位
	role    string   // 文件命名方式
	exts    []string // 允许的扩展名
}

// 声明一个Upload对象。
// dir 上传文件的保存目录，若目录不存在，则会尝试创建;
// maxSize 允许上传文件的最大尺寸，单位为byte；
// role 文件命名规则，格式可参考time.Format()参数；
// exts 允许的扩展名，若为空，将不允许任何文件上传。
func New(dir string, maxSize int64, role string, exts ...string) (*Upload, error) {
	// 确保所有的后缀名都是以.作为开始符号的。
	es := make([]string, 0, len(exts))
	for _, ext := range exts {
		if ext[0] != '.' {
			es = append(es, "."+ext)
			continue
		}
		es = append(es, ext)
	}

	// 确保dir最后一个字符为目录分隔符。
	last := dir[len(dir)-1]
	if last != '/' && last != filepath.Separator {
		dir = dir + string(filepath.Separator)
	}

	// 确保dir目录存在，若不存在则会尝试创建。
	stat, err := os.Stat(dir)
	if err != nil && !os.IsExist(err) {
		if !os.IsNotExist(err) {
			return nil, err
		}

		// 尝试创建目录
		if err = os.MkdirAll(dir, 0660); err != nil {
			return nil, err
		}

		// 创建目录成功，重新获取状态
		if stat, err = os.Stat(dir); err != nil {
			return nil, err
		}
	}
	if !stat.IsDir() {
		return nil, errors.New("dir不是一个目录")
	}

	return &Upload{
		dir:     dir,
		maxSize: maxSize,
		role:    role,
		exts:    es,
	}, nil
}

// 判断扩展名是否符合要求。
func (u *Upload) checkExt(ext string) bool {
	if len(ext) == 0 { // 没有扩展名，一律过滤
		return false
	}

	// 是否为允许的扩展名
	for _, e := range u.exts {
		if e == ext {
			return true
		}
	}
	return false
}

// 检测文件大小是否符合要求。
func (u *Upload) checkSize(file multipart.File) (bool, error) {
	var size int64

	switch f := file.(type) {
	case stater:
		stat, err := f.Stat()
		if err != nil {
			return false, err
		}
		size = stat.Size()
	case sizer:
		size = f.Size()
	default:
		return false, errors.New("上传文件时发生未知的错误")
	}

	return size <= u.maxSize, nil
}

// 设置水印，file为水印文件的路径，或是在isText为true时，file为水印的文字。
func (u *Upload) SetWaterMark(file string, isText bool) {
	// TODO
}

// 招行上传的操作。会检测上传文件是否符合要求，只要有一个文件不符合，就会中断上传。
// 返回的是相对于u.dir目录的文件名列表。
func (u *Upload) Do(field string, w *http.ResponseWriter, r *http.Request) ([]string, error) {
	r.ParseMultipartForm(32 << 20)
	heads := r.MultipartForm.File[field]
	ret := make([]string, len(heads))

	for _, head := range heads {
		file, err := head.Open()
		if err != nil {
			return nil, err
		}

		ext := filepath.Ext(head.Filename)
		if !u.checkExt(ext) {
			return nil, errors.New("包含无效的文件类型")
		}

		ok, err := u.checkSize(file)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, errors.New("超过最大的文件大小")
		}

		path := time.Now().Format(u.role) + ext
		ret = append(ret, path)
		f, err := os.Create(u.dir + path)
		if err != nil {
			return nil, err
		}

		io.Copy(f, file)

		f.Close()
		file.Close() // for的最后关闭file
	}

	return ret, nil
}