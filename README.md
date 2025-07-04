# AnyShape

**AnyShape** is a blazing-fast, concurrency-enabled string pattern search tool for directory trees. Designed for flexibility, speed, and extensibility, it scans files under a given path for **partial or full matches** of a search keyword — even across multiple combinations and variations of that keyword.

---

## 🚀 Features

- ✅ **Search any shape of a word**: intelligently generates combinations of the target word
- ⚡ **Concurrent processing**: fast file scanning using worker goroutines
- 📂 **Recursive directory scanning**
- 🔍 **Case-insensitive matching** (soon)
- 📄 **Line-level and character-level match reporting**
- 🛑 **Optional exclusions for specific combinations**
- 💡 **Customizable worker count**
- 🔍 **Customizable Search engine, allowing to search only file/folder names, only contents or both**
- ✅ **Organized results without repetitive matches on the same positions of the same file**

---

## 📦 Use Case Examples

AnyShape is perfect for:

- Developers hunting down usage of a function or keyword in large codebases
- Security researchers scanning for sensitive terms in logs or dumps
- Codebase migration and refactoring tasks
- Any scenario requiring partial/fuzzy word matching across files

---

## 🧠 How It Works

The search engine intelligently breaks the `targetWord` into **multiple lowercase character combinations**, starting from the full word down to 3-letter fragments. For each file under the `address`, it searches each line for those combinations and reports every match with file path, line number, and character index.

---

## 📥 Installation

```bash
git clone https://github.com/pya-h/anyshape.git
cd anyshape
go build -o anyshape
````

---

## 🔧 Usage

```bash
anyshape <address> <targetWord> [workers] [-w] [-x <exclude1> <exclude2> ...] [+fn/-fn] [+ct/-ct]
```

### Required:

* `address`: Root directory to scan
* `targetWord`: The main word or keyword to search for

### Optional:

* `workers`: Number of concurrent file-reading workers (defaults to logical CPU cores)
* `-w`: If passed, the search would be limited to word by word search; e.g. combinations like 'tes' does not match a word like 'Test'.
    It's obvious that this kind of search is faster, but the default search mode finds any type of occurrence.
* `-x`: Followed by a list of combinations that you don't want to search for.
* `+fn/-fn`: Enabling/Disabling file/folder name search [default: disabled]
* `+ct/-ct`: Enabling/Disabling file content search [default: enabled]

📌 **Order of flags is arbitrary** — you can place them in any position.

---

## 🧪 Example

```bash
anyshape ./logs TestWord 100 -x tes testwor .git -w
```

* Searches recursively under `./logs`
* Generates combinations from the word `TestWord`
* Uses 100 concurrent workers
* Excludes `tes` and `testwor` from combinations
* Writes matches to `matches.txt`

---

## 🗂 Output Format

Each match is printed in the following format:

```
match_text | relative/file/path.txt | line <n>, char <m>
```

And written to `anyshape-matches.txt` (if `-w` is provided) with the same format.



## 📌 Author

Developed by [pya-h](https://github.com/pya-h)