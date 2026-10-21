# 发布说明

每次发布前，在这里创建一个与 tag 同名的 Markdown 文件：

- tag `v0.1.0` → `.github/release-notes/v0.1.0.md`
- tag `v0.2.0` → `.github/release-notes/v0.2.0.md`

工作流会把该文件的内容作为 GitHub Release 正文，并在后面追加 GitHub
自动生成的提交列表。没有对应文件时，只显示自动生成的说明。

> 文件必须在打 tag **之前**提交，因为工作流 checkout 的是 tag 指向的提交。

示例：

~~~markdown
## 新增

- 支持自动检查更新

## 修复

- 修正译文复制失败的问题
~~~
