# Countlines

This is an automation script to count the number of lines of all files in a subdirectory. Also usable for github repos.  
It optimizes file parsing using multi-threading depending on physical thread count.  

## Optional Flags

1. `-help`:
    Show flag options.  

2. `-blacklist`:
    Allows blacklisting of directories / files to be excluded from the count.  

3. `-no-color`:
    Remove color output in the terminal.  

4. `-branch`:
    Allows specification of branch of github repo.  

5. `-commit`:
    Allows specification of commit of github repo.  

6. `-output-csv`:
    Creates a csv file (output.csv) in current directory to store results.  

## How to use

1. Have the Go Compiler.

    ```bash
    go version
    ```

2. Clone the repo

    ```bash
    git clone https://github.com/SidoJain/Countlines.git
    ```

3. Build the project

    ```bash
    go build
    ```

4. Add to ~/.bashrc

    ```bash
    # add directory to path
    export PATH="<directory_path>:$PATH"

    # make a simple alias
    alias countlines='Countlines.exe'
    ```

5. Use anywhere

    ```bash
    countlines [-flags] <directory/URL> [pattern1] [pattern2] ...
    ```
