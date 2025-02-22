# 防火墙规则自动更新

自动将服务器相关端口开放给本机IP，相较服务器直接在公网裸奔，这样做会更安全

目前支持腾讯云轻量，阿里云ECS(本人只有这两台)，计划适配阿里云腾讯云的轻量+ECS和华为云Flexus

## 使用方法

配置node和npm环境，安装方法参考[Node.js 安装配置 | 菜鸟教程](https://www.runoob.com/nodejs/nodejs-install-setup.html)

修改配置文件相关信息，并填入token，地域，主机/防火墙ID，默认的规则是为本地开放所有端口

```bash
mv config.example.json config.json
```

然后安装依赖

```bash
npm install
```

运行

```bash
node index.js
```

可以添加定时规则自动更新

假设项目路径位于 `/home/user/UpdateFirewall`

```bash
crontab -e
*/30 * * * * /usr/bin/node /home/user/UpdateFirewall/index.js
```
