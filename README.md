# Run GO code from a string  

It's a terminal program for a testing speed of another GO program as a string or a file.

It is impossible to run GO
code as a string inside a Go program directly (http://stackoverflow.com/a/27680056/1024794 and
http://stackoverflow.com/a/28784016/1024794). So I do it saving second GO string to a file, compile it (`go
build`), and run through bash command from a first Go program.

## How to Run
The program can be run from console: `go run /path/to/go_run_go/goRunGo.go`

For short GO code (less than 1000 chars) it can be inputted into the console as a string and as an `/absolute/path/to/file` for long text.
For long GO code (more than 1000 chars) it may be inputted only by a filename. 

After inputting a GO code, the user sets **maximum execution time** in milliseconds for the string.
When the program is finished, the system gives the actual execution time in the terminal. If the code runs over maximum execution time, it will be stopped and show the result.

Several GO strings after each other are possible to input as the program works with each GO code in async mode.


Errors in the second GO code string will not break the program, they  will just be shown as a result. 




## Cache
Each second GO code is saved to cache, so if it's run the second time, the program  will not write it to disk and will not compile it again, it will just run old compiled executive file.


There is a cache for storing second GO code source and compiled files. It is placed in `~/SecondGoStore` folder.
It has a limit of `100 megabytes`. After reaching this limit, the cache will be cleaned automatically the following way:

First it tries to delete files and folders older than 2 days. If the cache is still too big, it starts deleting the oldest directories one by one until cache size becomes less than the limit.

