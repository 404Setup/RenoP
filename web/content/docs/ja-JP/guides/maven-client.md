---
title: Maven と Gradle
order: 1
category: ガイド
description: 公開ドメインの検証と Maven / Gradle クライアント設定
---

# Maven / Gradle クライアント設定

Maven リポジトリを作成し、アカウントメニューで成果物の逆ドメイン名前空間を作成・検証します。公開ドメインと
L0-L4 チームはすべての Maven リポジトリで共有されます。読み取りには公開範囲（Visibility）、公開にはリポジトリの書き込み権限とドメインの
公開レベルの両方が必要です。

自動化（CI/CD）には `repository:read` や `repository:publish` 権限を持つ有効期限付き API トークンの利用を推奨します。HTTP Basic 認証の
ユーザー名はアカウント名、パスワードには API トークンを指定します。

## Maven

### 依存関係解決 (`pom.xml`)

```xml
<repositories>
    <repository>
        <id>renop-releases</id>
        <name>RenoP Releases</name>
        <url>https://packages.example.com/releases</url>
        <releases>
            <enabled>true</enabled>
        </releases>
        <snapshots>
            <enabled>false</enabled>
        </snapshots>
    </repository>
</repositories>
```

Snapshot が必要な場合は 2 つ目のリポジトリ定義を追加します。`HIDDEN` は正確な URL で解決できますが一覧には表示されず、`PRIVATE`
の読み取りには認証情報が必要です。

### 公開先 (`pom.xml`)

```xml
<distributionManagement>
    <repository>
        <id>renop-releases</id>
        <name>RenoP Releases</name>
        <url>https://packages.example.com/releases</url>
    </repository>
</distributionManagement>
```

`groupId` は公開者が管理する検証済みドメインの配下である必要があります。クラシックとモダンのレイアウトは同じクライアント URL と
公開規則を使用します。

### 認証情報 (`~/.m2/settings.xml`)

```xml
<settings>
    <servers>
        <server>
            <id>renop-releases</id>
            <username>alice</username>
            <password>rnp_pat_REDACTED</password>
        </server>
    </servers>
</settings>
```

`<id>` は `pom.xml` の `<id>` と完全一致させます。認証情報はプロジェクト外で管理し、CI の機密管理から注入してください。

---

## Gradle

### 依存関係解決 (`build.gradle.kts`)

```kotlin
repositories {
    maven {
        name = "renopReleases"
        url = uri("https://packages.example.com/releases")
        credentials {
            username = providers.gradleProperty("renopUser").get()
            password = providers.gradleProperty("renopToken").get()
        }
    }
}
```

### 公開 (`build.gradle.kts`)

```kotlin
plugins {
    `maven-publish`
}

publishing {
    repositories {
        maven {
            name = "renop"
            url = uri("https://packages.example.com/releases")
            credentials {
                username = providers.gradleProperty("renopUser").get()
                password = providers.gradleProperty("renopToken").get()
            }
        }
    }
    publications {
        create<MavenPublication>("mavenJava") {
            from(components["java"])
        }
    }
}
```

`renopUser` と `renopToken` はユーザーの Gradle プロパティまたは CI の機密情報に保存し、ソース管理には含めないでください。

## Javadoc の表示

有効な `*-javadoc.jar` が存在し事前確認が有効な場合、パスやファイルサイズの上限内でサンドボックスビューアに展開されます。

URL: `https://packages.example.com/javadoc/{repo}/{group}/{artifact}/{version}/index.html`

Javadoc の有無によって認可規則が変わることはありません。画面上の署名済み表示は、ファイル名ではなくバックエンドの GPG 署名記録に基づいて判定されます。
