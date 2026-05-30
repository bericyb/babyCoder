# Directory Test

The directory test is used to test the ability of a harness and its agent to navigate and create file systems and use bash to manipulate them according to natural language instructions. 

# Test 1: Create multiple of the same specified directories in each of the sub-directories of the current directory. 
Prompt: in each of the different subdirectories in this folder please create folders named:
claude-code
opencode
pi
cline
cursor
antigravity
babyCoder
Leave the directories empty. 

### Tree output before:
.
├── documents
├── email
├── server
├── services
├── tests
└── tickets

7 directories, 0 files
### Tree output after:
.
├── documents
│   ├── antigravity
│   ├── babyCoder
│   ├── claude-code
│   ├── cline
│   ├── cursor
│   ├── opencode
│   └── pi
├── email
│   ├── antigravity
│   ├── babyCoder
│   ├── claude-code
│   ├── cline
│   ├── cursor
│   ├── opencode
│   └── pi
├── server
│   ├── antigravity
│   ├── babyCoder
│   ├── claude-code
│   ├── cline
│   ├── cursor
│   ├── opencode
│   └── pi
├── services
│   ├── antigravity
│   ├── babyCoder
│   ├── claude-code
│   ├── cline
│   ├── cursor
│   ├── opencode
│   └── pi
├── tests
│   ├── antigravity
│   ├── babyCoder
│   ├── claude-code
│   ├── cline
│   ├── cursor
│   ├── opencode
│   └── pi
└── tickets
    ├── antigravity
    ├── babyCoder
    ├── claude-code
    ├── cline
    ├── cursor
    ├── opencode
    └── pi

49 directories, 0 files
