# redislock-gozero 🔒  
_A Go-Zero compatible Redis distributed lock._

[![Go Reference](https://pkg.go.dev/badge/github.com/<your-username>/redislock-gozero.svg)](https://pkg.go.dev/github.com/<your-username>/redislock-gozero)
[![Go Report Card](https://goreportcard.com/badge/github.com/<your-username>/redislock-gozero)](https://goreportcard.com/report/github.com/<your-username>/redislock-gozero)

---

## 🚀 Overview

`redislock-gozero` is a lightweight distributed locking library for Go, fully compatible with  
[`github.com/zeromicro/go-zero/core/stores/redis`](https://github.com/zeromicro/go-zero).

It’s based on the logic of [bsm/redislock](https://github.com/bsm/redislock),  
but rewritten to use **go-zero’s Redis client** instead of `go-redis/v9`.

This makes it ideal for **go-zero microservices** or any application already using `go-zero`'s core Redis layer.

---

## ✨ Features

- 🔒 Safe distributed lock with automatic expiration  
- 🔁 Configurable retry/backoff strategies  
- 🕒 Lock TTL introspection & refresh support  
- 🧠 Context-aware operations  
- ⚡ Built on top of go-zero’s efficient Redis client

---

## 📦 Installation

```bash
go get github.com/Salehisaac/redislock-gozero.git
