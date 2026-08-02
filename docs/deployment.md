# 官网部署

`site/` 是一个 vinext 网站，当前没有数据库、登录、上传或运行时密钥。最简单可靠的
自有服务器方案是以 Node.js 服务运行构建产物，并由现有 Caddy 或 Nginx 负责 HTTPS
和反向代理。

## 构建

```bash
node scripts/sync-product-content.mjs
cd site
npm ci
npm run build
npm test
```

生产服务器使用 Node.js 22.13 或更高版本。在服务器中保留 `site/package.json`、锁文件、
依赖和构建产物，然后执行项目的生产启动命令：

```bash
cd site
npm ci
npm run start
```

vinext 的生产启动器当前由项目锁文件统一安装，因此服务器阶段保留完整的锁定依赖。
将服务绑定到回环地址，再由反向代理把域名转发到该端口。进程可交给 systemd、
supervisord、Docker Compose 或服务器已有的进程管理器；优先复用现有运维方式。

## 自动部署所需信息

要由维护者或自动化直接部署到自有服务器，需要提供：

- SSH 主机、端口、用户名和认证方式；
- 目标目录以及该用户的写入权限；
- 正式域名、DNS 是否已指向服务器；
- 当前使用 Caddy、Nginx、Docker 还是其他进程管理方式；
- 服务监听端口和防火墙约束；
- 是否允许部署脚本重启该网站服务。

当前网站不需要业务环境变量。若日后加入统计、下载镜像或错误上报，应把相关变量写入
服务器密钥管理，不提交到仓库。

## 推荐发布流程

1. CI 完成内容同步检查、网站构建和渲染测试。
2. 把已验证的 `site/` 源码和构建产物上传到带时间戳的新目录。
3. 在新目录安装生产依赖并启动临时端口，先做本机健康检查。
4. 原子切换 `current` 软链接或反向代理目标并重载服务。
5. 保留上一版本，验证失败时切回，不在原目录覆盖部署。

## 验证

部署后至少检查：

```bash
curl -fI https://example.com/
curl -fI https://example.com/features
curl -fI https://example.com/guide
curl -fI https://example.com/download
curl -fI https://example.com/changelog
curl -fI https://example.com/favicon.ico
```

随后在桌面与移动端目视确认导航、16:10 完整截图、下载状态、版本号和 Open Graph
分享图。只有域名、TLS、页面和静态资源均实际验证通过后，才能记录为部署成功。
