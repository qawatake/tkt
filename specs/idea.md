# idea

- 現状作った、作ろうとしているgrep, query, rm, editは人間向け。AI向けの機能性を作りたい。

## commands

### query

#### 入力

sql clientやclaude codeのようにnoninteractiveなモードを提供すればよさそう。

#### 出力

jsonかなぁ。

### grep

#### 入力


#### 出力

### rm

- インタラクティブで。
- エンターを押すと、rmとマークされる。
- でもファイルがいきなり消えることはない。pushやdiffで削除を認識される。
- 例えば、ファイル名の冒頭にxxx_とかをつけるとか？？これはclaudeと相談したい。

### edit

- インタラクティブで。
- エンターを押すと、editorが起動する。(vim)
- 終了するともとのリストビューにもどる。

## idea

- treeコマンドを実際のフォルダ構造以外で出力する。
  - 例えば、sprint viewとかtimeline view(parent keyをもとにした構造)とか。(まぁ、順序性をjiraと同期するのはとても難しいのだが。。。)
- tkt grep -sとかすると、まずスプリントについてヒアリングできるとかあると便利？？
- プラグイン機構があると便利？？tkt openとかtkt editは結局jqとかviを組み合わせるだけで良さそうなので。`tkt-open`を探す感じ。
- llm向けコマンド？
  - tree? ls? query? grep? fd?(ファイルパスも)
  - rm (noninteractive)
