import type { BaseLayoutProps } from 'fumadocs-ui/layouts/shared';

export function baseOptions(): BaseLayoutProps {
  return {
    nav: {
      title: (
        <div className="flex items-center gap-2">
          <img 
            src="/goflow-logo.png" 
            alt="GoFlow" 
            className="h-7 object-contain rounded"
            style={{ aspectRatio: '1198/1494' }}
          />
          <span className="font-bold bg-gradient-to-r from-[#5B8DEE] to-[#7C5BBF] bg-clip-text text-transparent">
            GoFlow
          </span>
        </div>
      ),
    },
    links: [
      {
        text: 'Documentation',
        url: '/docs',
        active: 'nested-url',
      },
      {
        text: 'GitHub',
        url: 'https://github.com/goflow/goflow',
        external: true,
      },
    ],
    githubUrl: 'https://github.com/goflow/goflow',
  };
}
