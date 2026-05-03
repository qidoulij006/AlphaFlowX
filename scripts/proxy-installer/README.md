# Proxy Installer

这个目录用于生成一个可上传到新云主机执行的 SOCKS5 代理安装包。

## 包含内容

- `install_socks5_proxy.sh`
  交互式安装脚本。输入用户名、密码、端口后，会自动完成：
  - 安装依赖
  - 下载并编译 `microsocks`
  - 写入 systemd 服务
  - 开机自启
  - 尝试放行防火墙端口
  - 本地自检代理出口

## 目标机器使用方式

上传并解压安装包后执行：

```bash
sudo bash install_socks5_proxy.sh
```

也可以非交互执行：

```bash
sudo bash install_socks5_proxy.sh \
  --username nofx \
  --password 'your-strong-password' \
  --port 1080 \
  --non-interactive
```

安装完成后，脚本会直接输出可填到 NOFX 的 `proxy_url`，格式类似：

```text
socks5://nofx:your-strong-password@47.91.11.229:1080
```

## 要求

- Linux 云主机
- root 权限
- 主机能访问公网下载依赖和 `microsocks` 源码
- 云厂商安全组已放行对应端口

## 已测试适配逻辑

脚本内置了以下包管理器检测：

- `apt`
- `dnf`
- `yum`
- `apk`

## 常用排查

查看服务状态：

```bash
systemctl status microsocks
```

查看日志：

```bash
journalctl -u microsocks -n 100 --no-pager
```

重启服务：

```bash
systemctl restart microsocks
```
