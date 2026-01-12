---
layout: home

hero:
  name: GoFlow
  text: The Go Alternative to LangChain & Inngest
  tagline: High-performance AI orchestration with agents, workflows, and queues. Self-hosted, zero vendor lock-in.
  image:
    src: /logo.svg
    alt: GoFlow
  actions:
    - theme: brand
      text: Get Started
      link: /guide/getting-started
    - theme: alt
      text: Why GoFlow?
      link: /guide/what-is-goflow
    - theme: alt
      text: GitHub
      link: https://github.com/goflow/goflow

features:
  - icon: ⚡
    title: 10x Faster Than Python
    details: Compiled Go performance. 5ms agent startup vs 2000ms. Handle 50,000 req/sec.
  - icon: 📦
    title: 3 Dependencies
    details: Minimal attack surface. No dependency hell. Compare to LangChain's 100+ packages.
  - icon: 💰
    title: 100% Free & Self-Hosted
    details: MIT licensed. No platform fees. No vendor lock-in. Run on your own infrastructure.
  - icon: 🤖
    title: Complete Agent Framework
    details: Tools, memory, streaming, multi-agent, consensus, and hierarchies. All built-in.
  - icon: 🔄
    title: Production Workflows
    details: DAGs, conditions, loops, parallel, retries, human approvals, and signals.
  - icon: 📊
    title: Built-in Job Queues
    details: Redis/DragonflyDB backend. Sharding, DLQ, cron, webhooks, and metrics.
---

<style>
.comparison-table {
  margin: 2rem auto;
  max-width: 900px;
}
.comparison-table table {
  width: 100%;
  border-collapse: collapse;
}
.comparison-table th, .comparison-table td {
  padding: 0.75rem;
  text-align: left;
  border-bottom: 1px solid var(--vp-c-divider);
}
.comparison-table th {
  font-weight: 600;
}
.section-title {
  text-align: center;
  margin: 3rem 0 1rem;
  font-size: 1.75rem;
  font-weight: 600;
}
.section-subtitle {
  text-align: center;
  color: var(--vp-c-text-2);
  margin-bottom: 2rem;
}
</style>

<div class="section-title">Why Choose GoFlow?</div>
<div class="section-subtitle">See how GoFlow compares to popular alternatives</div>

<div class="comparison-table">

| Feature | GoFlow | LangChain | Inngest AgentKit |
|---------|--------|-----------|------------------|
| **Language** | Go 🚀 | Python 🐢 | TypeScript |
| **Performance** | 50,000 req/sec | 500 req/sec | N/A (SaaS) |
| **Cold Start** | 5ms | 2,000ms | N/A |
| **Dependencies** | 3 | 100+ | 20+ |
| **Self-Hosted** | ✅ 100% | ✅ | ⚠️ Requires cloud |
| **Cost** | Free (MIT) | Free | $$$ at scale |
| **Agents** | ✅ | ✅ | ✅ |
| **Workflows** | ✅ Built-in | ❌ External | ⚠️ Via platform |
| **Job Queues** | ✅ Built-in | ❌ Need Celery | ⚠️ Via platform |
| **Dashboard** | ✅ Built-in | ❌ | ⚠️ Via platform |
| **Multi-Agent** | ✅ Advanced | ✅ | ✅ |
| **MCP Servers** | ✅ | ❌ | ✅ |
| **Vendor Lock-in** | None | None | Inngest platform |

</div>

<div class="section-title">What You Get</div>
<div class="section-subtitle">Everything you need in one package</div>

```
┌─────────────────────────────────────────────────────────────────────────┐
│                              GoFlow                                      │
├─────────────────┬─────────────────┬─────────────────┬───────────────────┤
│     Agents      │    Workflows    │     Queues      │   Integrations    │
│                 │                 │                 │                   │
│ • Multi-LLM     │ • DAG Engine    │ • Sharded       │ • OpenAI          │
│ • Tool System   │ • Conditionals  │ • Dead Letter   │ • Anthropic       │
│ • Memory        │ • Loops         │ • Cron Jobs     │ • Gemini          │
│ • Streaming     │ • Parallel      │ • Webhooks      │ • MCP Servers     │
│ • Multi-Agent   │ • Approvals     │ • Metrics       │ • E2B Sandbox     │
│ • Consensus     │ • Signals       │ • Rate Limit    │ • Browserbase     │
└─────────────────┴─────────────────┴─────────────────┴───────────────────┘
```

## Quick Start

```bash
go get github.com/goflow/goflow
```

```go
package main

import (
    "context"
    "fmt"
    
    "github.com/goflow/goflow/pkg/agent"
    "github.com/goflow/goflow/pkg/llm/openai"
    "github.com/goflow/goflow/pkg/tools"
)

func main() {
    // Create LLM (reads OPENAI_API_KEY from env)
    llm := openai.New("")
    
    // Create agent with tools
    myAgent := agent.New(llm, tools.BuiltinTools())
    
    // Run a task
    result, _ := myAgent.Run(context.Background(), 
        "Search the web for GoFlow and summarize what you find")
    
    fmt.Println(result.Output)
}
```

## Supported LLM Providers

| Provider | Models | Streaming |
|----------|--------|-----------|
| **OpenAI** | GPT-4o, GPT-4o-mini, o1-preview | ✅ |
| **Anthropic** | Claude 3.5 Sonnet, Claude 3 Opus | ✅ |
| **Google Gemini** | Gemini 1.5 Pro (2M context!) | ✅ |

## Deploy Anywhere

```bash
# Docker
docker run -e OPENAI_API_KEY=sk-... goflow/goflow

# Kubernetes
kubectl apply -f https://goflow.dev/k8s.yaml

# Single binary
./goflow serve --port 8080
```
