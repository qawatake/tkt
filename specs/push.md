# push

- ローカルにあるファイルとキャッシュを比較し、違いがあるファイルについて処理を行う。
- 違いがあるファイルをremoteからfetchしてキャッシュを最新化する。
- 改めて差分を比較する。
- 差分があったファイルに対して、差分をユーザーにpagerで見せたうえで、適用するかどうかy/Nで確認する。
- 適用すると回答されたファイルはremoteにpushする。(非同期で。)

## その他の仕様

- readonlyな属性は比較もしないし、書き込みもしない。具体的には、body、タイトル、type、parentKey以外はreadonly

## todo

1. [x] pushしたあとにキャッシュを更新する。(つまり、pushしたあとにdiffを実行しても差分がないようにする。)
2. [x] 現状はpushが同期なのでconcを使って非同期化する。
3. [x] push前のfetchをチケット一つ一つfetchしていて無駄なので、idsでまとめて問い合わせるようにする。1ページに収まらなければconcで非同期化する。ただし、使うAPIはhttps://developer.atlassian.com/cloud/jira/platform/rest/v3/api-group-issues/#api-rest-api-3-issue-bulkfetch-postを使うこと。分割してconcで非同期化する。既存のSearchメソッドが参考になるはず。
4. [x] 現状はkeyがwritable、タイトルがreadonlyになっている。keyはreadonlyにして、タイトルはwritableにする。つまり、keyは変更できないが、タイトルは変更できるようにする。
5. [x] `push -f`で強制的にpushできるようにする。
