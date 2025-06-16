# initコマンド

https://github.com/ankitpokhrel/jira-cli と同じように設定をインタラクティブに作成します。

1. 設定ファイル`ticket.yml`をカレントディレクトリに作成します。
2. Installation typeはCloud決め打ちです。(jira cliの確認事項)
3. link to jira serverは聞きましょう。
4. login emailも聞きましょう。
5. link to jira serverとlogin emailからプロジェクトにアクセスできるはずです。default projectを選択肢から選ばせましょう。
6. プロジェクトが決まるとデフォルトのボードも選ばせましょう。これも選択肢から選ばせましょう。
5. jqlを聞きましょう。デフォルトは`project = <default project>`です。
7. 以上です。

## todo

1. [x] ticket.ymlの対応。
2. [x] `1-1`みたいな謎の番号を消す。例: `ボードを選択してください (1-1): 1`。`プロジェクトを選択してください (1-2): 1`。
3. [x] loadingにはgithub.com/briandowns/spinnerを使う。
4. [x] jqlも聞く。そしてデフォルトは`project = <default project>`にする。
5. [x] 現状はmdファイルを格納するディレクトリ(mergeコマンドの出力先)は`tmp`で決め打ち。これをのticket.ymlで設定できるようにする。
6. [x] ボードやプロジェクトを選ぶときはpecoのようにfuzzy searchできるようにする。
7. [x] taskやエピックの情報を取得する。(要するにissue typeの情報を取得する。)
8. [x] 設定ファイル名を`ticket.yml`から`tkt.yml`に変更する。
9. [x] デフォルトのjqlにupdatedの条件も追加して、大量のチケットが取得されないようにする。
10. [ ] issue typeを取得するのに利用するAPIを"/rest/api/3/issuetype"ではなく、"/rest/api/3/project/{projectId}"に変更する。
