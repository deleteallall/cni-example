实现一个简单 cni plugin，该 plugin 在 Pod 的 network namespace 和宿主机的
network namespace 间通过一对 veth 联通，实现 pod 与宿主机的互访。

使用方法：

1. 在每个节点上，将编译后的二进制文件和 shell 脚本放在 /op/cni/bin/ 下  
2. 在每个节点上，将 01-example01.conf 文件放在 /etc/cni/net.d/ 下
3. 操作 kubectl 创建 Pod
4. 在宿主机和 Pod 互相 ping
