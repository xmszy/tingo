# 文件上传

Tingo 提供文件上传处理，基于 `multipart/form-data`。

## 单文件上传

~~~go
func (u *Upload) Image(c *t.Ctx) error {
    // 获取上传文件
    file, err := c.FormFile("file")
    if err != nil {
        return errors.New(400, "UPLOAD_ERROR", "请选择文件")
    }

    // 保存到本地
    dst := filepath.Join("public/uploads", file.Filename)
    if err := c.SaveUploadedFile(file, dst); err != nil {
        return errors.New(500, "UPLOAD_FAIL", "文件保存失败")
    }

    c.JSON(200, t.Map{
        "url":  "/uploads/" + file.Filename,
        "name": file.Filename,
        "size": file.Size,
    })
    return nil
}
~~~

## 多文件上传

~~~go
func (u *Upload) Images(c *t.Ctx) error {
    form, err := c.MultipartForm()
    if err != nil {
        return err
    }
    files := form.File["images"]

    var urls []string
    for _, file := range files {
        dst := filepath.Join("public/uploads", file.Filename)
        if err := c.SaveUploadedFile(file, dst); err != nil {
            continue
        }
        urls = append(urls, "/uploads/"+file.Filename)
    }

    c.JSON(200, t.Map{"urls": urls})
    return nil
}
~~~

## 文件验证

上传前验证文件类型和大小：

~~~go
func (u *Upload) Image(c *t.Ctx) error {
    file, err := c.FormFile("file")
    if err != nil {
        return err
    }

    // 检查大小（10MB）
    if file.Size > 10<<20 {
        return errors.New(400, "FILE_TOO_LARGE", "文件不能超过10MB")
    }

    // 检查类型
    ext := strings.ToLower(filepath.Ext(file.Filename))
    allowedExts := map[string]bool{
        ".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".webp": true,
    }
    if !allowedExts[ext] {
        return errors.New(400, "FILE_TYPE_DENY", "不支持的文件类型")
    }

    dst := filepath.Join("public/uploads", fmt.Sprintf("%d%s", time.Now().UnixNano(), ext))
    if err := c.SaveUploadedFile(file, dst); err != nil {
        return err
    }

    c.JSON(200, t.Map{"url": "/uploads/" + filepath.Base(dst)})
    return nil
}
~~~

## upload 组件

`tingo-contrib/upload`（独立模块）提供带校验的上传辅助，零外部依赖：

~~~go
package upload

import (
    "github.com/xmszy/tingo-contrib/upload"
)

func (u *Upload) Image(c *t.Ctx) error {
    // Save 校验并保存单个文件：大小上限 + 扩展名白名单
    err := upload.Save(c, "file", "public/uploads/avatar.png", upload.Config{
        MaxSize:   10 << 20,                  // 10MB
        AllowExts: []string{".jpg", ".png", ".gif"},
    })
    if err != nil {
        if errors.Is(err, upload.ErrTooLarge) {
            return errors.New(400, "FILE_TOO_LARGE", "文件不能超过10MB")
        }
        if errors.Is(err, upload.ErrExtNotAllowed) {
            return errors.New(400, "FILE_TYPE_DENY", "不支持的文件类型")
        }
        return err
    }

    c.JSON(200, t.Map{"url": "/uploads/avatar.png"})
    return nil
}
~~~

~~~go
// SaveAll 批量保存同名多文件，任一校验失败立即停止
paths, err := upload.SaveAll(c, "images", "public/uploads/", upload.Config{
    MaxSize:   5 << 20,
    AllowExts: []string{".jpg", ".png", ".webp"},
})
if err != nil { /* ... */ }
// paths = ["public/uploads/a.jpg", "public/uploads/b.png"]
~~~

| 配置项 | 类型 | 说明 |
|---|---|---|
| `MaxSize` | `int64` | 单文件最大字节数，`0` 不限制 |
| `AllowExts` | `[]string` | 允许的扩展名（含点），空表示不限制 |
