# Dream Memory System - Visual Architecture

## System Flow Diagram

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                          SESSION START                                       │
└─────────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
                    ┌───────────────────────────────┐
                    │  Check for .babycoder/dream.txt │
                    └───────────────────────────────┘
                                    │
                    ┌───────────────┴───────────────┐
                    ▼                               ▼
            ┌───────────────┐              ┌──────────────┐
            │  File exists   │              │  No file     │
            └───────────────┘              └──────────────┘
                    │                               │
                    ▼                               ▼
        ┌──────────────────────┐          ┌────────────────┐
        │  Read dream content   │          │  Start empty   │
        └──────────────────────┘          └────────────────┘
                    │                               │
                    ▼                               │
    ┌──────────────────────────────┐              │
    │  Inject into system prompt   │              │
    │  as "PROJECT CONTEXT"        │              │
    └──────────────────────────────┘              │
                    │                               │
                    ▼                               │
        ┌────────────────────┐                    │
        │  Show: 💭 Project  │                    │
        │  memory loaded     │                    │
        └────────────────────┘                    │
                    │                               │
                    └───────────────┬───────────────┘
                                    ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                       INTERACTIVE LOOP                                       │
└─────────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
                        ┌───────────────────┐
                        │  You: [message]   │
                        └───────────────────┘
                                    │
                                    ▼
                    ┌───────────────────────────┐
                    │  Cancel any pending timer │
                    └───────────────────────────┘
                                    │
                                    ▼
                        ┌───────────────────┐
                        │  Agent processes  │
                        │  and responds     │
                        └───────────────────┘
                                    │
                                    ▼
                    ┌───────────────────────────┐
                    │  Start 10-second timer    │
                    └───────────────────────────┘
                                    │
                    ┌───────────────┴───────────────┐
                    ▼                               ▼
        ┌────────────────────┐          ┌─────────────────────┐
        │  User types again  │          │  10 seconds pass    │
        │  (within 10s)      │          │  (no input)         │
        └────────────────────┘          └─────────────────────┘
                    │                               │
                    ▼                               ▼
        ┌────────────────────┐          ┌─────────────────────┐
        │  Timer cancelled   │          │  Trigger dream      │
        │  (back to loop)    │          │  update (async)     │
        └────────────────────┘          └─────────────────────┘
                    │                               │
                    │                               ▼
                    │               ┌─────────────────────────────┐
                    │               │  DREAM UPDATE PROCESS       │
                    │               │  (runs in background)       │
                    │               └─────────────────────────────┘
                    │                               │
                    │                               ▼
                    │               ┌─────────────────────────────┐
                    │               │  Check message count        │
                    │               │  (need > 2 messages)        │
                    │               └─────────────────────────────┘
                    │                               │
                    │               ┌───────────────┴────────────┐
                    │               ▼                            ▼
                    │   ┌──────────────────┐        ┌───────────────────┐
                    │   │  ≤ 2 messages    │        │  > 2 messages     │
                    │   │  Skip update     │        │  Continue         │
                    │   └──────────────────┘        └───────────────────┘
                    │                                           │
                    └───────────────────────────────────────────┤
                                                                ▼
                                            ┌───────────────────────────────┐
                                            │  STEP 1: SUMMARIZE SESSION    │
                                            └───────────────────────────────┘
                                                                │
                                                                ▼
                                            ┌───────────────────────────────┐
                                            │  Get last 10 messages         │
                                            │  Format: "role: content"      │
                                            └───────────────────────────────┘
                                                                │
                                                                ▼
                                            ┌───────────────────────────────┐
                                            │  LLM Call #1:                 │
                                            │  "Summarize in 2-3 sentences" │
                                            └───────────────────────────────┘
                                                                │
                                                                ▼
                                            ┌───────────────────────────────┐
                                            │  Result: sessionSummary       │
                                            │  "User added auth feature..." │
                                            └───────────────────────────────┘
                                                                │
                                                                ▼
                                            ┌───────────────────────────────┐
                                            │  STEP 2: READ CURRENT DREAM   │
                                            └───────────────────────────────┘
                                                                │
                                                                ▼
                                            ┌───────────────────────────────┐
                                            │  Read .babycoder/dream.txt    │
                                            │  (or empty if doesn't exist)  │
                                            └───────────────────────────────┘
                                                                │
                                                                ▼
                                            ┌───────────────────────────────┐
                                            │  STEP 3: DECIDE UPDATE        │
                                            └───────────────────────────────┘
                                                                │
                                                                ▼
                                            ┌───────────────────────────────┐
                                            │  LLM Call #2:                 │
                                            │  Current: [dream]             │
                                            │  Session: [summary]           │
                                            │  Should update?               │
                                            └───────────────────────────────┘
                                                                │
                                            ┌───────────────────┴────────────────┐
                                            ▼                                    ▼
                                ┌──────────────────────┐        ┌────────────────────────┐
                                │  Response:           │        │  Response:             │
                                │  "NO_UPDATE"         │        │  [Updated dream text]  │
                                └──────────────────────┘        └────────────────────────┘
                                            │                                    │
                                            ▼                                    ▼
                                ┌──────────────────────┐        ┌────────────────────────┐
                                │  Keep current dream  │        │  Write new dream.txt   │
                                │  (no file change)    │        │  (atomic write)        │
                                └──────────────────────┘        └────────────────────────┘
                                            │                                    │
                                            └────────────────┬───────────────────┘
                                                             ▼
                                            ┌────────────────────────────────┐
                                            │  Dream update complete         │
                                            │  (or error logged silently)    │
                                            └────────────────────────────────┘
                                                             │
                                                             ▼
                                            ┌────────────────────────────────┐
                                            │  Back to interactive loop      │
                                            │  (user sees no interruption)   │
                                            └────────────────────────────────┘
```

## Timing Diagram

```
Time →
────────────────────────────────────────────────────────────────────────────

t=0s        User: "Add authentication"
            │
            ▼
t=1s        Agent: "I've added authentication to user.go"
            │
            ▼
t=1s        [10-second timer starts]
            │
            │ ... user reviewing output ...
            │
            ├─────────────── SCENARIO A: Quick Response ──────────────────┐
            │                                                              │
t=5s        │ User types: "Now add tests"                                 │
            │ [Timer cancelled]                                            │
            │                                                              │
            │ Agent responds...                                            │
            │ [New 10-second timer starts]                                 │
            │                                                              │
            └──────────────────────────────────────────────────────────────┘

            ├─────────────── SCENARIO B: Idle Update ─────────────────────┐
            │                                                              │
t=5s        │ ... user still thinking ...                                 │
            │                                                              │
t=10s       │ ... user still thinking ...                                 │
            │                                                              │
t=11s       │ [Timer fires - dream update starts async]                   │
            │                                                              │
            │ ┌─ Background Process ────────────────────────────┐         │
            │ │ Summarize session        (2s)                   │         │
t=13s       │ │ Read dream file          (instant)              │         │
            │ │ Decide update            (2s)                   │         │
t=15s       │ │ Write dream.txt          (instant)              │         │
            │ └─────────────────────────────────────────────────┘         │
            │                                                              │
t=15s       │ [Dream updated, user unaware]                               │
            │                                                              │
t=20s       │ User types: "Thanks!"                                        │
            │                                                              │
            └──────────────────────────────────────────────────────────────┘
```

## State Machine

```
                    ┌────────────────┐
                    │  AGENT_IDLE    │
                    └────────────────┘
                            │
                            │ User message received
                            ▼
                    ┌────────────────┐
                    │  AGENT_RUNNING │◄────┐
                    └────────────────┘     │
                            │              │
                            │ Response     │ Tool calls
                            │ complete     │
                            ▼              │
                    ┌────────────────┐     │
                    │  TIMER_WAITING │─────┘
                    └────────────────┘
                            │
                    ┌───────┴────────┐
                    │                │
        User input  │                │  10 seconds elapsed
                    ▼                ▼
            ┌────────────┐   ┌─────────────────┐
            │ CANCEL     │   │ DREAM_UPDATING  │
            │ (back to   │   │ (async)         │
            │  RUNNING)  │   └─────────────────┘
            └────────────┘            │
                                      │ Update complete
                                      ▼
                              ┌────────────────┐
                              │  AGENT_IDLE    │
                              │  (ready)       │
                              └────────────────┘
```

## Component Interaction

```
┌────────────────────────────────────────────────────────────────────────┐
│                            main.go                                     │
│                                                                        │
│  ┌──────────────────────────────────────────────────────────────┐    │
│  │  runInteractive()                                             │    │
│  │                                                               │    │
│  │  1. Load dream.txt                                           │    │
│  │  2. Inject into system prompt                                │    │
│  │  3. Create agent with projectRoot                            │    │
│  │  4. Interactive loop                                          │    │
│  └──────────────────────────────────────────────────────────────┘    │
└────────────────────────────────────────────────────────────────────────┘
                                    │
                                    │ creates
                                    ▼
┌────────────────────────────────────────────────────────────────────────┐
│                    internal/services/agent/agent.go                    │
│                                                                        │
│  ┌──────────────────────────────────────────────────────────────┐    │
│  │  Agent struct                                                 │    │
│  │                                                               │    │
│  │  Fields:                                                      │    │
│  │  • messages        []Message                                 │    │
│  │  • dreamTimer      *time.Timer                               │    │
│  │  • projectRoot     string                                    │    │
│  └──────────────────────────────────────────────────────────────┘    │
│                                                                        │
│  ┌──────────────────────────────────────────────────────────────┐    │
│  │  Run(ctx, executor)                                           │    │
│  │  • Cancel existing timer                                      │    │
│  │  • Process messages                                           │    │
│  │  • On success: start 10s timer → updateDream()               │    │
│  └──────────────────────────────────────────────────────────────┘    │
│                                                                        │
│  ┌──────────────────────────────────────────────────────────────┐    │
│  │  updateDream(ctx)                                             │    │
│  │  1. Check: > 2 messages?                                      │    │
│  │  2. Call summarizeSession()                                   │    │
│  │  3. Read dream.txt                                            │    │
│  │  4. Call decideDreamUpdate()                                  │    │
│  │  5. Write file if not NO_UPDATE                               │    │
│  └──────────────────────────────────────────────────────────────┘    │
│                                                                        │
│  ┌──────────────────────────────────────────────────────────────┐    │
│  │  summarizeSession(ctx)                                        │    │
│  │  • Get last 10 messages                                       │    │
│  │  • Format for LLM                                             │    │
│  │  • Return: "User did X, changed Y"                            │    │
│  └──────────────────────────────────────────────────────────────┘    │
│                                                                        │
│  ┌──────────────────────────────────────────────────────────────┐    │
│  │  decideDreamUpdate(ctx, currentDream, summary)                │    │
│  │  • Compare current vs session                                 │    │
│  │  • Ask LLM: update needed?                                    │    │
│  │  • Return: "NO_UPDATE" or [new dream text]                    │    │
│  └──────────────────────────────────────────────────────────────┘    │
└────────────────────────────────────────────────────────────────────────┘
                                    │
                                    │ writes to
                                    ▼
┌────────────────────────────────────────────────────────────────────────┐
│                      Filesystem                                        │
│                                                                        │
│  .babycoder/                                                          │
│  └── dream.txt                                                        │
│      └── "babyCoder is a Go-based AI agent for local LLMs..."        │
│                                                                        │
│  Atomic write: temp file → rename (prevents corruption)               │
└────────────────────────────────────────────────────────────────────────┘
```

## Data Flow

```
┌─────────────┐
│   User      │
│   Input     │
└──────┬──────┘
       │
       ▼
┌─────────────────────┐
│   Agent processes   │
│   (tool calls, etc) │
└──────┬──────────────┘
       │
       ▼
┌──────────────────────┐     ┌─────────────────┐
│  Agent.messages[]    │────▶│  Last 10 msgs   │
│  (in memory)         │     │  extracted      │
└──────────────────────┘     └────────┬────────┘
                                      │
                                      ▼
                            ┌──────────────────┐
                            │  LLM Provider    │
                            │  (summarize)     │
                            └────────┬─────────┘
                                     │
                                     ▼
                          ┌─────────────────────┐
                          │  Session Summary    │
                          │  "Added auth to..." │
                          └──────────┬──────────┘
                                     │
        ┌────────────────────────────┴────────────────────┐
        │                                                  │
        ▼                                                  ▼
┌─────────────────┐                          ┌──────────────────────┐
│  dream.txt      │─────────────────────────▶│  Current Dream       │
│  (filesystem)   │      Read                │  (in memory)         │
└─────────────────┘                          └──────────┬───────────┘
        ▲                                               │
        │                                               │
        │                                               ▼
        │                                    ┌────────────────────┐
        │                                    │  LLM Provider      │
        │                                    │  (decide update)   │
        │                                    └──────────┬─────────┘
        │                                               │
        │                                               │
        │                              ┌────────────────┴──────────────────┐
        │                              │                                   │
        │                              ▼                                   ▼
        │                    ┌──────────────────┐              ┌────────────────┐
        │                    │  "NO_UPDATE"     │              │  Updated Dream │
        │                    │  (do nothing)    │              │  Text          │
        │                    └──────────────────┘              └────────┬───────┘
        │                                                               │
        └───────────────────────────────────────────────────────────────┘
                            Write (atomic)
```

## Error Handling Flow

```
┌────────────────────────────────────────────────────────────────┐
│                    Error Scenarios                             │
└────────────────────────────────────────────────────────────────┘

updateDream() called
        │
        ├─→ messages.length ≤ 2 ────────────────→ Return early (skip)
        │
        ├─→ projectRoot == "" ──────────────────→ Return early (skip)
        │
        ├─→ summarizeSession() fails ───────────→ Log error, return
        │                                          User unaffected
        │
        ├─→ ReadFile(dream.txt) fails ──────────→ Use empty string
        │                                          (first time is OK)
        │
        ├─→ decideDreamUpdate() fails ──────────→ Log error, return
        │                                          User unaffected
        │
        ├─→ LLM returns "NO_UPDATE" ────────────→ Skip write (normal)
        │
        └─→ WriteFile(dream.txt) fails ─────────→ Log error, return
                                                   User unaffected

Result: All errors handled gracefully, never interrupt user
```

## Concurrency Model

```
Main Thread (Interactive Loop)                Background Timer Thread
─────────────────────────────────            ─────────────────────────

User types message
      │
      ▼
Cancel any pending timer ─────────────────X──→ [Timer cancelled]
      │
      ▼
Agent.Run() starts
      │
      ▼
Process message
      │
      ▼
Agent responds
      │
      ▼
Start 10s timer ───────────────────────────┬─→ Timer created
      │                                     │
      ▼                                     │
Ready for next input                       │ ... 10 seconds ...
      │                                     │
      │                                     ▼
      │                                   Timer fires
      │                                     │
      │                                     ▼
      │                                   updateDream()
      │                                     │
      │                                     ├─→ summarizeSession()
      │                                     │   (LLM call #1)
      │                                     │
      │                                     ├─→ Read dream.txt
      │                                     │
      │                                     ├─→ decideDreamUpdate()
      │                                     │   (LLM call #2)
      │                                     │
      │                                     └─→ Write dream.txt
      │                                         
User types again                           [Done, thread exits]
      │
      ▼
Cancel timer (no-op if already done)
      │
      ▼
Agent.Run() starts
      ...

Note: No mutex needed - timer callback doesn't access shared state
      Only touches filesystem and makes LLM calls
```

---

## Key Takeaways

1. **Simplicity**: Single timer, three functions, one file
2. **Async**: Updates never block user interaction
3. **Smart**: LLM decides what's important
4. **Reliable**: All errors handled gracefully
5. **Tested**: 11 comprehensive unit tests

The dream system is self-contained, minimal, and elegant. 🌙
