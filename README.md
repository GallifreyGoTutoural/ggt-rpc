# ggt-rpc
[English](README.md) | [简体中文](README_zh.md)

ggt-rpc is an RPC framework implemented from scratch based on the official standard library net/rpc of Go language, and adds features such as protocol exchange, registration center, service discovery, load balancing, and timeout processing on this basis.

The "ggt" in the project name stands for "Gallifrey's GoTutorial

The main reference source of the project is the blog of Geek Tutu: [7 days to implement RPC framework GeeRPC from scratch with Go](https://geektutu.com/post/geerpc.html). If you want to learn more about program design details and considerations, please refer to the original blog.

## Development plan

- [ ] Use encoding/gob to implement message encoding and decoding
- [ ] Implement a simple server
- [ ] Implement a high-performance client that supports asynchronous and concurrent
- [ ] Implement service registration function through reflection
- [ ] Implement service call on the server side
- [ ] Add connection timeout processing mechanism
- [ ] Add timeout processing mechanism for server processing
- [ ] Support HTTP protocol
- [ ] Implement server-side load balancing through random selection and Round Robin polling scheduling algorithm
- [ ] Implement a simple registration center that supports service registration, receiving heartbeats, etc.
- [ ] The client implements a service discovery mechanism based on the registration center
