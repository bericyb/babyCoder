# babyCoder Agent Loop

End-to-end flow of a prompt through babyCoder: initialization, the agent loop,
tool execution, and the post-session dream/memory update.

## Flow Diagram

```mermaid
flowchart TD
    Start([User runs babyCoder]) --> Mode{Args provided?}
    Mode -->|No| Interactive[runInteractive]
    Mode -->|Yes| NonInteractive[runNonInteractive]

    Interactive --> Init
    NonInteractive --> Init

    subgraph Init [initializeAgentContext]
        direction TB
        LoadCfg[Load config<br/>.babycoder/babycoder.json] --> OpenDB[Open SQLite<br/>.babycoder/babycoder.db]
        OpenDB --> NewProv[Create AI Provider]
        NewProv --> NewSess[Create Session UUID]
        NewSess --> BuildPrompt[Build system prompt:<br/>main_agent + rules + dream.txt]
        BuildPrompt --> RegTools[Register tools:<br/>read_file, write_file,<br/>line_edit, find_replace,<br/>list_files, bash_execute,<br/>get_project_structure,<br/>check_code_status, test_status]
    end

    Init --> Input{Mode}
    Input -->|Interactive| ReadLine[Read stdin line]
    Input -->|Non-interactive| UseArg[Use args as prompt]

    ReadLine --> SpecialCmd{Special?}
    SpecialCmd -->|# rule| AddRule[rulesManager.AddRule] --> ReadLine
    SpecialCmd -->|/exit| Exit([Exit])
    SpecialCmd -->|prompt| AddUser

    UseArg --> AddUser[agent.AddUserMessage<br/>saveMessage to DB]
    AddUser --> RunLoop

    subgraph RunLoop [agent.Run - loop up to MaxIterations]
        direction TB
        BuildReq[Build ChatCompletionRequest<br/>messages + tools + auto] --> CallLLM[provider.ChatCompletion]
        CallLLM --> SaveAsst[Append assistant message<br/>persist to DB<br/>update session timestamp]
        SaveAsst --> Finish{FinishReason?}

        Finish -->|stop| Done[Schedule dream timer 10s]
        Finish -->|tool_calls| ExecTools

        subgraph ExecTools [For each tool_call]
            direction TB
            Parse[Unmarshal arguments JSON] --> Exec[toolExecutor &rarr; registry.Execute]
            Exec --> Track[Record timing, success,<br/>error, file_path]
            Track --> SaveExec[SaveToolExecution to DB]
            SaveExec --> AppendTool[Append role=tool message<br/>with tool_call_id<br/>persist to DB]
        end

        ExecTools --> BuildReq
    end

    Done --> PostRun
    PostRun[testRunner.RunIfDirty<br/>if build/test command known] --> PrintAsst[Print last assistant content]

    PrintAsst -->|Interactive| ReadLine
    PrintAsst -->|Non-interactive| DreamNow[UpdateDreamNow]

    DreamNow --> DreamFlow

    subgraph DreamFlow [Dream Update]
        direction TB
        Sum[summarizeSession:<br/>LLM summarizes last 10 msgs] --> ReadDream[Read .babycoder/dream.txt]
        ReadDream --> Decide[decideDreamUpdate:<br/>LLM compares + emits<br/>NO_UPDATE or new summary]
        Decide --> Write{Update?}
        Write -->|Yes| WriteFile[Write dream.txt]
        Write -->|No| Skip[skip]
    end

    DreamFlow --> Exit
```

## Key Points

- **Single loop** in `agent.Run` (`internal/services/agent/agent.go:130`):
  request &rarr; LLM &rarr; if `tool_calls`, execute all, append results, repeat;
  if `stop`, exit.
- **Persistence** happens at every step (assistant message, tool execution,
  tool response message) via `storage.Database`.
- **Dream memory** runs post-session (`internal/services/agent/dream.go:15`)
  via two LLM calls (summarize + decide-update) and persists to
  `.babycoder/dream.txt`, which is injected into the next session's system
  prompt.
