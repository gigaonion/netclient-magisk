# netclient-magisk

root化されたAndroid環境において，NetmakerのP2Pメッシュネットワーククライアントである`netclient`を自律的なシステムデーモンとして稼働させるためのモジュール実装です．

標準Linux向け実装が依存するネットワーク管理機構を排除し，Androidの`netd`制約をバイパスする専用のポリシーベースルーティングおよびSplit DNSをカーネルレベルで直接制御します．

> [!IMPORTANT]
> 前提条件とカーネル仕様
> - OS / 権限: Android 16 (KernelSUまたはMagisk導入済み環境)
> - カーネル: Linux Kernel 5.6+
> - 認証モデル: ブラウザベースのOIDC認証は無効化されており，Enrollment Tokenによる登録のみをサポートします．

> [!NOTE]
> 永続化パスについて
> Androidのファイルシステム構造に合わせて，設定ファイル，鍵ペア，およびログはすべて`/data/adb/netclient/`に保存されます．

---

## 使い方

モジュールをフラッシュして，Enrollment-Tokenを保存すれば，通常のNetmakerと同様に動作します．

### WebUIから登録する
1. KernelSU / Magiskアプリのモジュール一覧からNetmaker ClientのWebUIを開きます．
2. Netmakerサーバー管理画面で発行したEnrollment Tokenを入力し，「登録実行」を押します．

### CLIから登録する
```bash
su -c netclient register -t "<Enrollment-Token>"
```

### 状態の確認

KernelSUアプリのモジュール詳細画面でActionボタンを押すと，現在のデーモン稼働状態，WireGuardインターフェース，PBRテーブル（Table 1000），および優先度ルール（pref 99）が出力されます．

```bash
# 手動でステータスを確認する場合
su -c /data/adb/modules/netclient-magisk/action.sh
```

---

## ビルド方法

### 必要環境
- Go 1.22+
- `zip` コマンド

```bash
./scripts/build_module.sh

# 出力先: dist/netclient-magisk.zip
```

---

## ライセンス&謝辞

### ライセンス
本リポジトリ内のコードは[Apache License 2.0](./LICENSE)のもとで公開されています．

本プロジェクトは[Gravitl / Netclient](https://github.com/gravitl/netclient)（Copyright Gravitl, Inc.）をフォーク・改変したものであり，Apache License 2.0のライセンス条項に基づき，Android環境向けにソースコードおよびネットワーク制御機構の変更を行っています．改変内容の詳細はコミット履歴をご参照ください．


### 謝辞
本プロジェクトは以下のオープンソースソフトウェアを利用・改変して作成されています．
- [Gravitl / Netclient](https://github.com/gravitl/netclient) - Gravitl, Inc.
- [WireGuard](https://www.wireguard.com/) - Jason A. Donenfeld
