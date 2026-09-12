---
title: 仓库与镜像
order: 2
category: 配置
description: 仓库引擎、可见性、上游镜像、迁移与 S3 存储
---

# 仓库与镜像

仓库定义存储在数据库中，通过仓库管理页面编辑。首次升级时，仅在数据库尚无仓库配置的情况下，
RenoP 才导入 `repositories.yaml`（或 `RENOP_REPOSITORIES` 指定的文件），成功后将源文件归档为
`repositories.yaml.migrated.<id>`。无效配置会停止启动并保留源文件。已有数据库配置始终优先，包括空仓库集合。
全新安装的仓库集合为空，不会生成 YAML 文件。请备份数据库，并保护可能含有 S3 和镜像凭据的
迁移归档。仓库名称是不可变的小写 slug，并作为 URL 的第一个路径段。

新安装的仓库列表为空，需要在管理页面显式创建仓库。已有数据库快照及一次性旧配置迁移会保留原先配置的仓库。

## 旧配置迁移示例

```yaml
repositories:
  releases:
    name: releases
    format: maven
    visibility: PUBLIC
    allow_redeployment: false
    require_gpg_signature: true
    publication_review: every_version
    download_statistics: true
    mirrors: []
  crates:
    name: crates
    format: cargo
    visibility: PUBLIC
    mirrors: []
  containers:
    name: containers
    format: docker
    visibility: PRIVATE
    allow_redeployment: false
    mirrors: []
```

## 仓库字段

| 字段                    | 默认值     | 说明                                                                      |
|:------------------------|:-----------|:--------------------------------------------------------------------------|
| `name`                  | 必填       | 不可变仓库 slug 与 URL 前缀                                               |
| `format`                | `maven`    | `maven`, `maven-classic`, `files`, `npm`, `cargo`, `docker`, `conan`, `conda`, `conda-native`, `apk`, `apt`, `rpm`, `yum` |
| `visibility`            | `PUBLIC`   | `PUBLIC`、`HIDDEN` 或 `PRIVATE`                                           |
| `allow_redeployment`    | `false`    | 在支持的引擎中允许 Maven 版本重发或 files/Docker 覆盖                     |
| `require_gpg_signature` | `false`    | Maven 发布要求通过 OpenPGP 分离签名校验                                   |
| `publication_review`    | `off`      | Maven/npm/Cargo/Docker 审核策略：`off`、`new_packages` 或 `every_version` |
| `download_statistics`   | 引擎默认值 | Maven/npm/Cargo/Docker 默认启用；`files` 需手动启用                       |
| `mirrors`               | `[]`       | 按顺序执行的上游镜像定义                                                  |
| `s3`                    | 省略       | 当前仓库独立的 S3 兼容存储                                                |

对于 npm 与 Docker，`new_packages` 会在占用名称前审核显式创建申请；`every_version` 还会审核之后的每个版本或
Manifest。Maven 与 Cargo 没有空包创建步骤，因此其 `new_packages` 策略审核首次发布。所有引擎的镜像源导入
均不进入审核流程。

`maven-classic` 只改变前端布局，仍执行 Maven 发布规则。`files` 为非结构化存储，不生成校验文件、POM 或
签名校验。Maven 可迁移到 `files` 并反向迁移，存储对象保持原位；切回 Maven 时重建目录并恢复保存的策略。
迁移前生效的下载统计开关会保持不变。

`files` 仓库的上传和镜像下载会保留相邻文件，即使路径包含 `SNAPSHOT`，或名称以 `.md5`、`.asc`、
`-javadoc.jar` 结尾。旧版本清理和 Javadoc 解压仅适用于 Maven 仓库。

发布审核支持 Maven、npm、Cargo、Docker 及受管理的原生资源。Maven 会将 `allow_redeployment` 强制设为 `false`；npm 保持不可变版本与
dist-tag 的事务。本地文件在仓库版主或系统管理员批准前保持隐藏，镜像内容不会进入审核。存在待审核发布时，
不能修改、删除仓库或迁移引擎。

`npm` 仓库要求先预留软件包再发布，保存不可变语义化版本与发布标签，支持作用域私有包、L0-L4 团队，
并可按精确软件包名称或 `@scope/*` 规则使用镜像。

### 可见性

- **PUBLIC**：允许匿名读取与发现。
- **HIDDEN**：匿名用户和无权限用户的目录中不显示，个人资料成员关系也不会公开。超级管理员和拥有显式仓库
  浏览权限的用户可以发现该仓库；精确已知的文件路径仍可读取。
- **PRIVATE**：读取、列表与写入均要求显式授权。私有 Docker 镜像还会检查镜像级 L0-L4 成员关系。

## 上游镜像

本地对象缺失时，RenoP 可按顺序从已启用镜像流式获取。成功结果可直接持久化，无需将完整正文读入内存。
Cargo 与 Docker 在适用上游名称已存在时会拒绝本地创建。

```yaml
mirrors:
  - name: "central"
    url: "https://repo1.maven.org/maven2"
    persist: true
    cache_ttl_secs: 86400
    negative_cache: true
    timeout_secs: 30
    proxy: ""
    allow_artifacts: []
    deny_artifacts: []
```

| 字段              | 默认值  | 说明                                  |
|:------------------|:--------|:--------------------------------------|
| `name`            | 必填    | 当前仓库内唯一的镜像名称              |
| `url`             | 必填    | 上游基础 URL                          |
| `persist`         | `true`  | 将成功响应写入仓库存储后端            |
| `cache_ttl_secs`  | `86400` | 正缓存有效时间                        |
| `negative_cache`  | `true`  | 缓存引擎支持的上游未命中结果          |
| `timeout_secs`    | `30`    | 单次上游请求超时                      |
| `proxy`           | `""`    | 使用全局路由、`direct` 或精确命名代理 |
| `allow_artifacts` | `[]`    | 按引擎解释的允许规则                  |
| `deny_artifacts`  | `[]`    | 按引擎解释的拒绝规则，拒绝优先        |

需要凭据时使用结构化 authorization 字段，不得将密钥嵌入 `url`。

## S3 兼容存储

每个仓库可使用 Disk 或独立 S3 兼容后端。切换存储或引擎时，分片仓库门控会与上传、删除、GPG 提交及镜像
写入进行串行化。

```yaml
s3:
  enabled: true
  endpoint: "https://s3.us-east-1.amazonaws.com"
  bucket: "my-renop-bucket"
  key_prefix: "releases/"
  region: "us-east-1"
  access_key_id: "YOUR_ACCESS_KEY"
  secret_access_key: "YOUR_SECRET_KEY"
  force_path_style: false
  redirect_downloads: false
```

MinIO 通常要求 `force_path_style`。启用 `redirect_downloads` 后，RenoP 完成授权并返回短时预签名跳转；否则
由 RenoP 流式代理对象。

`capacity_limit_bytes` 是单个存储库的已安装字节上限，`0` 表示不限；界面使用 MiB。Disk 和 S3 的软件包、生成的校验文件、待审核对象及镜像缓存均计入，临时暂存副本不计入。提交前预留容量，并发上传共享同一上限。超限写入返回 `507` 和 `repository_capacity_exceeded`；有效的镜像响应仍可直接传输，但不写入缓存。上限降至已有用量以下不会删除文件或阻止读取。通过 RenoP 之外的方式修改存储后，请重建索引或重启以重新统计。旧客户端省略该可选字段时保留原上限。

## 原生软件包仓库

Conan 通过原生客户端支持配方和二进制包的修订版本。Conda 和 Conda native 接受位于 `noarch/` 等平台目录中的 `.conda` 与 `.tar.bz2` 包，并生成 `repodata.json`。APK 在 `x86_64/` 等架构目录中为软件包生成 `APKINDEX.tar.gz`。apt 接受 `.deb` 文件，生成 `Packages`、`Packages.gz` 和 `dists/<suite>/Release`，并将 `pool/<component>/` 映射到对应组件。rpm/yum 生成 `repodata/repomd.xml` 及其引用的元数据。

上传前先在仓库浏览器中登记托管原生资源。资源标识来自包元数据或 Conan 配方引用，无需全局团队前缀。L0 读取，L1 发布，L2 替换或删除版本，L3 管理设置与成员，L4 拥有资源；必须保留至少一名 L4 所有者。只有仓库写权限并不能修改他人的资源。原生镜像仓库保持只读。

`new_packages` 审核登记后的首次发布，`every_version` 审核每个原生版本或 Conan 修订。签名不完整和待审核文件不会进入下载或自动索引，重启后也一样。APK 必须配置发布者 RSA PEM 公钥，并提供覆盖控制段与负载的有效原生签名；RPM 必须提供受信任的 OpenPGP 签名，直接覆盖负载或通过已签名的负载摘要验证。在资源设置中配置公钥。APT 通过签名仓库索引认证，无需为每个 `.deb` 上传分离签名；Conda 使用原生包哈希，不额外要求 `.asc` 文件。

新上传不能替换自动生成的仓库索引；已有旧元数据会保留到管理员移除。每份自动索引最多 10,000 个包，更大集合应拆分为多个仓库或原生子目录。单个包上传上限为 8 GiB，每个资源最多保留 256 个未发布文件，实例总计最多 16,384 个。已有但未登记的文件仍可读取，冲突的托管上传替换前须由管理员先行处理。

自动生成的 APK、apt 和 rpm 元数据使用私有设置数据库中持久保存的独立密钥签名。对应公钥分别位于 `renop.rsa.pub`、`renop.asc` 和 `repodata/repomd.xml.key`。apt 还提供 `InRelease` 和 `Release.gpg`，rpm 提供 `repomd.xml.asc`。使用仓库浏览器中的客户端命令前，应先导入公钥。rpm 软件包本身的签名仍由发布者负责，需要另外导入对应发布者的密钥。

Conan 使用官方签名扩展清单和 OpenPGP 签名。将 `scripts/conan/sign.py` 安装到 `<CONAN_HOME>/extensions/plugins/sign/sign.py`，在资源中登记公钥，将 `RENOP_CONAN_GPG_KEY` 设为私钥指纹，将 `RENOP_CONAN_GPG_KEYRING` 设为供 `gpgv` 使用的可信去装甲公钥环。清单中的所有文件与签名收齐前保持隐藏。对已发布 Conan 文件的完全相同内容重试是幂等的。

```sh
conan cache sign "PACKAGE/*"
conan upload "PACKAGE/*" --remote "REPOSITORY" --confirm
```


## 共享文件内容

内容相同的 Disk 文件通过硬链接共享不可变内容，同时保留各自的逻辑路径。写入会原子替换文件，删除一个路径不会影响其他引用。单个后台任务分批扫描存量文件，跳过繁忙仓库，并在批次之间暂停。可更新的索引、暂存文件和有过期时间的 Disk 镜像缓存保持独立文件。

S3 对至少 64 KiB 的内容使用指向共享对象的逻辑引用，更小的文件仍直接保存。哈希和引用持久保存在私有文件索引中，已知记录可避免重复探测元数据。缺少可信哈希的旧对象会暂时保留，直到正常上传或经 RenoP 完整下载时顺带获得哈希；后台去重不会仅为比较内容而下载这些对象。重建索引时可能使用 LIST 分页，并对缺失的引用元数据进行一次校验。日常回收处理索引中的待回收队列，不会反复扫描整个桶。

完整的 S3 备份应同时包含仓库对象、私有 `.renop-content-v1` 命名空间和私有索引。独立的 RenoP 部署应使用不同的键前缀。无引用的内容会在宽限期后回收。仓库容量和下载统计继续按逻辑文件大小计算，不因物理共享而减少。
