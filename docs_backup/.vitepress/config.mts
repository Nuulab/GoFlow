import { defineConfig } from 'vitepress'

export default defineConfig({
  title: 'GoFlow',
  description: 'AI Orchestration Framework for Go',

  themeConfig: {
    logo: '/logo.svg',
    
    nav: [
      { text: 'Guide', link: '/guide/getting-started' },
      { text: 'API', link: '/api/agent' },
      { text: 'Examples', link: '/examples/basic-agent' },
      { text: 'GitHub', link: 'https://github.com/goflow/goflow' }
    ],

    sidebar: {
      '/guide/': [
        {
          text: 'Introduction',
          items: [
            { text: 'What is GoFlow?', link: '/guide/what-is-goflow' },
            { text: 'Getting Started', link: '/guide/getting-started' },
            { text: 'Installation', link: '/guide/installation' },
          ]
        },
        {
          text: 'Core Concepts',
          items: [
            { text: 'Agents', link: '/guide/agents' },
            { text: 'LLM Providers', link: '/guide/providers' },
            { text: 'Tools', link: '/guide/tools' },
            { text: 'Workflows', link: '/guide/workflows' },
            { text: 'Queues', link: '/guide/queues' },
            { text: 'Webhooks', link: '/guide/webhooks' },
          ]
        },
        {
          text: 'Advanced',
          items: [
            { text: 'Microservice Setup', link: '/guide/microservice' },
            { text: 'Multi-Agent', link: '/guide/multi-agent' },
            { text: 'Integrations', link: '/guide/integrations' },
            { text: 'Scaling', link: '/guide/scaling' },
            { text: 'Deployment', link: '/guide/deployment' },
          ]
        }
      ],
      '/api/': [
        {
          text: 'Packages',
          items: [
            { text: 'agent', link: '/api/agent' },
            { text: 'workflow', link: '/api/workflow' },
            { text: 'queue', link: '/api/queue' },
            { text: 'tools', link: '/api/tools' },
            { text: 'cache', link: '/api/cache' },
            { text: 'api', link: '/api/api-server' },
          ]
        },
        {
          text: 'SDK',
          items: [
            { text: 'JavaScript', link: '/api/js-sdk' },
            { text: 'CLI', link: '/api/cli' },
          ]
        }
      ],
      '/examples/': [
        {
          text: 'Examples',
          items: [
            { text: 'Basic Agent', link: '/examples/basic-agent' },
            { text: 'Custom Tools', link: '/examples/custom-tools' },
            { text: 'Workflow', link: '/examples/workflow' },
            { text: 'Multi-Agent', link: '/examples/multi-agent' },
          ]
        }
      ]
    },

    socialLinks: [
      { icon: 'github', link: 'https://github.com/goflow/goflow' }
    ],

    footer: {
      message: 'Released under the MIT License.',
      copyright: 'Copyright © 2024 GoFlow'
    },

    search: {
      provider: 'local'
    }
  }
})
