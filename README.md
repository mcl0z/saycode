
  # SayCoding

  SayCoding is a terminal-first AI coding assistant built around
  multi-agent collaboration.

  Instead of treating coding as a single linear chat, SayCoding
  allows a lead agent to break work into subtasks, spawn child
  agents in parallel, coordinate them in real time, grant
  permissions when needed, and merge the results back into a
  final response.

  It is designed for real coding workflows where planning,
  editing, testing, and coordination often need to happen at the
  same time.
                                                                   ## Highlights

  - Hierarchical multi-agent collaboration
  - Parallel task execution                                        - Live lead-agent coordination
  - Delegated write and shell/test permissions                     - Resumable sessions
  - Terminal-native UI with agent activity visibility
  - File editing, patching, diffing, grep/glob search, and shell
  execution

  ## Why SayCoding

  Most terminal AI coding tools behave like a single assistant
  with a long conversation history.

  SayCoding takes a different approach:                          
  - The lead agent can split work into multiple parallel subtasks
  - Child agents can ask questions during execution
  - The lead agent can stay active while child agents are running
  - Permissions can be granted selectively instead of giving
  every agent full access
  - The user can watch coordination in the terminal UI

  This makes it more suitable for medium to large coding tasks,
  especially when work can be decomposed across files or
  concerns.

  ## Multi-Agent Model

  SayCoding uses a hierarchical agent model.

  ### Lead Agent

  The lead agent is responsible for:

  - understanding the user request
  - planning and decomposition
  - spawning child agents                                          - coordinating messages between agents
  - granting permissions
  - resetting stuck agents
  - producing the final integrated answer

  ### Child Agents                                               
  Child agents handle concrete subtasks such as:

  - reading and analyzing code
  - implementing isolated changes
  - testing specific modules                                       - reporting findings back to the lead
  - requesting permissions when needed

  ### Grandchild Agents

  Child agents may spawn one additional layer of agents.

  The maximum nesting depth is:

  - Lead Agent
  - Child Agent
  - Grandchild Agent

  Grandchild agents cannot spawn more agents.

  This keeps the system powerful without allowing uncontrolled
  recursive delegation.

  ## Permission Model

  SayCoding uses delegated permissions for safer parallel work.

  By default:

  - the lead agent has full access
  - child agents start read-only
  - child agents cannot edit files unless the lead grants write
  access
  - child agents cannot run shell/test commands unless the lead
  grants shell access

  Child agents can request access by sending a message to the
  lead agent during execution.

  The lead agent can then decide whether to grant:

  - file write access
  - shell/test access

  This allows collaboration without giving every agent
  unrestricted authority.                                        
  ## Real-Time Coordination

  One of SayCoding’s core ideas is that coordination should
  remain active while parallel work is running.

  During multi-agent execution:

  - child agents can send messages to the lead
  - the lead can respond while they are still working
  - the lead can grant permissions online
  - the lead can reset a specific agent if it gets stuck
  - the user can observe this interaction in the activity view

  This makes the system feel less like isolated workers and more
  like a coordinated engineering team.
                                                                   ## Terminal UI

  The built-in terminal UI is designed to make multi-agent
  workflows visible and controllable.

  It includes:

  - live conversation rendering
  - tool call visibility
  - agent list with hierarchy
  - current status per agent
  - tool call counts per agent
  - token usage per agent
  - recent coordination activity
  - session list and session resume
  - command palette
  - optional auto-continue workflow modes
                                                                   ## Session Persistence

  SayCoding saves conversation state so work is easier to          recover.

  It supports:

  - automatic session saving
  - session resume
  - recent session browsing
  - session picker in the UI                                     
  This is especially useful for long-running coding tasks and      interrupted terminal sessions.
                                                                   ## Built-In Tooling

  SayCoding includes core coding tools for terminal workflows:   
  - `read_file`
  - `list_dir`
  - `search_files`
  - `grep_files`
  - `edit_file`
  - `write_file`                                                   - `apply_patch`
  - `show_diff`
  - `run_shell`

  For collaboration it also includes:                            
  - `send_message`
  - `read_inbox`                                                   - `list_agents`
  - `grant_permissions`                                            - `reset_agent`
  - `spawn_agents`

  ## Example Workflow

  1. The user asks for a non-trivial change.
  2. The lead agent reads the code and decomposes the task.        3. The lead spawns multiple child agents in parallel.
  4. A child agent discovers it needs to edit files or run tests.
  5. The child sends a message to the lead asking for permission.
  6. The lead reviews the request and grants access if
  appropriate.
  7. Agents continue working and report results back.              8. The lead merges the results and produces the final response.

  ## Good Fit                                                    
  SayCoding is a strong fit for:

  - multi-file refactors
  - codebase exploration plus implementation
  - parallel debugging
  - implementation plus verification workflows
  - large tasks that benefit from decomposition
  - long-running terminal coding sessions                        
  ## Design Goals

  The project is built around a few practical goals:

  - keep the interface terminal-native
  - make agent coordination visible
  - preserve control over permissions
  - support decomposition without losing coherence
  - keep sessions resumable
  - stay useful for real engineering work, not just demo chats

  ## Project Direction

  The current direction focuses on:

  - stronger multi-agent collaboration
  - clearer coordination visibility
  - safer delegated permissions
  - more robust editing tools
  - better terminal usability and performance

  Future work may include:

  - a fuller MCP system in Go
  - richer task and todo management
  - improved rendering performance
  - stronger coordination loops
  - deeper external tool and server integration

  ## Summary

  SayCoding is not just a terminal chat assistant.

  It is a terminal coding environment built around coordinated
  multi-agent execution.

  If you want a coding assistant that can:

  - split work
  - run tasks in parallel
  - coordinate online
  - delegate permissions
  - resume sessions
  - and show what the agent team is doing
                                                                   then SayCoding is built for that direction.
