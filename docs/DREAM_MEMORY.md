# Dream Memory System

The dream memory system provides automatic project context persistence across sessions through a simple, self-updating summary file.

## Overview

After each agent interaction, a 10-second idle timer starts. If the agent remains idle (no new user input), it:

1. Summarizes the recent session (last 10 messages)
2. Reads the current `.babycoder/dream.txt` file
3. Asks the LLM if the dream should be updated based on the session
4. Updates the dream file if necessary

## Dream File

**Location:** `.babycoder/dream.txt`

**Content:** 1-2 paragraphs describing what the project is and what it does, written from an agent's perspective.

**Example:**
```
babyCoder is a Go-based AI coding agent optimized for smaller local language models. 
It uses hashline-based code addressing for precise edits and SQLite for persistence. 
The architecture follows a structured monolith pattern with services separated in 
internal/services/. Currently includes a self-updating memory system for maintaining 
context across sessions.
```

## How It Works

### Session Start
- Agent reads `.babycoder/dream.txt` (if exists)
- Dream content is injected into system prompt as "PROJECT CONTEXT"
- Agent starts with full project understanding
- User sees: `💭 Project memory loaded`

### During Session
- Agent processes user requests normally
- After successful completion, 10-second idle timer starts
- If user types before timer expires, timer is cancelled
- New timer starts after next agent response

### After 10 Seconds of Idle
**Step 1: Session Summary**
- Agent reviews last 10 messages
- Generates 2-3 sentence summary focused on changes made

**Step 2: Dream Update Decision**
- Agent reads current dream file
- Compares with session summary
- Decides if update is warranted
- Returns either updated dream or "NO_UPDATE"

**Step 3: File Update**
- If update needed, writes new content to dream.txt
- Uses atomic write (temp file + rename) to prevent corruption
- Logs success/failure to `.babycoder/logs/`

## Timer Behavior

```
User Message → Agent Responds → [10s Timer Starts]
                                      ↓
                    ┌─────────────────┴─────────────────┐
                    ↓                                   ↓
           [User types again]                   [10s elapsed]
                    ↓                                   ↓
           [Timer cancelled]                    [Dream updates]
                    ↓
           [New timer after response]
```

## Update Strategy

The LLM decides when to update based on:
- **New features added** - Yes, update
- **Bug fixes** - Maybe update if significant
- **Trivial changes** - No update
- **Architectural decisions** - Yes, update
- **New files/modules** - Yes, update
- **Documentation only** - Usually no update

This keeps the dream focused on high-level project understanding, not every minor detail.

## Benefits

✅ **Zero manual effort** - Completely automatic  
✅ **Fast session resumption** - Context loaded instantly  
✅ **Token efficient** - Dream is ~200-500 tokens vs full history (10K+)  
✅ **Invisible to user** - Updates happen during natural pauses  
✅ **Self-maintaining** - LLM decides relevance, not hardcoded rules  

## Configuration

Currently hardcoded:
- Idle timeout: 10 seconds
- Session summary: Last 10 messages
- Dream size: 1-2 paragraphs (LLM-maintained)

Future: Add to `.babycoder/babycoder.json` for customization.

## Example Workflow

```bash
# Start session
./babyCoder
# Output: 💭 Project memory loaded

You: Add a new field to the User struct
Agent: [makes changes]
# [10 second timer starts]
# [User waits, thinking about what to do next]
# [Dream updates in background]

You: Now add validation for that field
Agent: [makes changes]
# [Timer starts again]

You: /exit
# Session ends, dream has been kept up to date
```

## Debugging

**Check if dream exists:**
```bash
cat .babycoder/dream.txt
```

**Watch dream updates in real-time:**
```bash
tail -f .babycoder/logs/babycoder_*.log | grep "Dream:"
```

**Test dream loading:**
```bash
echo "Test project for AI coding" > .babycoder/dream.txt
./babyCoder
# Should see: 💭 Project memory loaded
```

## Implementation Details

- **Timer pattern:** Single timer per agent, reset on new `Run()` call
- **Concurrency:** Updates run in goroutine, non-blocking
- **Error handling:** Failures logged silently, don't interrupt user experience
- **File safety:** Atomic writes prevent corruption
- **Minimum content:** Skips update if session has ≤2 messages

## Future Enhancements

Potential improvements:
- Initialize dream from README on first run (condensed by LLM)
- Add `./babyCoder memory show` CLI command
- Configure idle timeout in `.babycoder/babycoder.json`
- Track dream version/update count
- Export dream history for review

## Philosophy

The dream is **not**:
- ❌ A complete project history
- ❌ A detailed changelog
- ❌ Documentation for humans
- ❌ A task list or TODO tracker

The dream **is**:
- ✅ A high-level project summary
- ✅ For agents to quickly understand context
- ✅ Maintained automatically
- ✅ Focused on what matters for coding assistance

Think of it as: "If you had to explain this project to another developer in 2 minutes, what would you say?"
