# Countlines

This is an automation script to count the number of lines of all files in a subdirectory.  
It optimizes file parsing using multi-threading.  

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
    countlines <directory> [pattern1] [pattern2] ...
    ```
