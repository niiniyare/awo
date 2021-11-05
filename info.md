Translations:

* [한국어 문서](README_ko.md)
* [简体中文](README_zh.md)
* [正體中文](README_zh-TW.md)
* [简体中文](README_zh-CN.md) - ???
* [Français](README_fr.md)
* [日本語](README_ja.md)
* [Portuguese](README_ptBR.md)
* [Español](README_es.md)
* [Română](README_ro.md)
* [Русский](README_ru.md)
* [Türkçe](README_tr.md)

## Overview

This is a basic layout for Go application projects. It's **`not an official standard defined by the core Go dev team`**; however, it is a set of common historical and emerging project layout patterns in the Go ecosystem. Some of these patterns are more popular than others. It also has a number of small enhancements along with several supporting directories common to any large enough real world application.

**`If you are trying to learn Go or if you are building a PoC or a simple project for yourself this project layout is an overkill. Start with something really simple instead (a single `main.go` file and `go.mod` is more than enough).`** As your project grows keep in mind that it'll be important to make sure your code is well structured otherwise you'll end up with a messy code with lots of hidden dependencies and global state. When you have more people working on the project you'll need even more structure. That's when it's important to introduce a common way to manage packages/libraries. When you have an open source project or when you know other projects import the code from your project repository that's when it's important to have private (aka `internal`) packages and code. Clone the repository, keep what you need and delete everything else! Just because it's there it doesn't mean you have to use it all. None of these patterns are used in every single project. Even the `vendor` pattern is not universal.

With Go 1.14 [`Go Modules`](https://github.com/golang/go/wiki/Modules) are finally ready for production. Use [`Go Modules`](https://blog.golang.org/using-go-modules) unless you have a specific reason not to use them and if you do then you don’t need to worry about $GOPATH and where you put your project. The basic `go.mod` file in the repo assumes your project is hosted on GitHub, but it's not a requirement. The module path can be anything though the first module path component should have a dot in its name (the current version of Go doesn't enforce it anymore, but if you are using slightly older versions don't be surprised if your builds fail without it). See Issues [`37554`](https://github.com/golang/go/issues/37554) and [`32819`](https://github.com/golang/go/issues/32819) if you want to know more about it.

This project layout is intentionally generic and it doesn't try to impose a specific Go package structure.

This is a community effort. Open an issue if you see a new pattern or if you think one of the existing patterns needs to be updated.

If you need help with naming, formatting and style start by running [`gofmt`](https://golang.org/cmd/gofmt/) and [`golint`](https://github.com/golang/lint). Also make sure to read these Go code style guidelines and recommendations:
* https://talks.golang.org/2014/names.slide
* https://golang.org/doc/effective_go.html#names
* https://blog.golang.org/package-names
* https://github.com/golang/go/wiki/CodeReviewComments
* [Style guideline for Go packages](https://rakyll.org/style-packages) (rakyll/JBD)

See [`Go Project Layout`](https://medium.com/golang-learn/go-project-layout-e5213cdcfaa2) for additional background information.

More about naming and organizing packages as well as other code structure recommendations:
* [GopherCon EU 2018: Peter Bourgon - Best Practices for Industrial Programming](https://www.youtube.com/watch?v=PTE4VJIdHPg)
* [GopherCon Russia 2018: Ashley McNamara + Brian Ketelsen - Go best practices.](https://www.youtube.com/watch?v=MzTcsI6tn-0)
* [GopherCon 2017: Edward Muller - Go Anti-Patterns](https://www.youtube.com/watch?v=ltqV6pDKZD8)
* [GopherCon 2018: Kat Zien - How Do You Structure Your Go Apps](https://www.youtube.com/watch?v=oL6JBUk6tj0)

A Chinese Post about Package-Oriented-Design guidelines and Architecture layer
* [面向包的设计和架构分层](https://github.com/danceyoung/paper-code/blob/master/package-oriented-design/packageorienteddesign.md)
