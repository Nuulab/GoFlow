import type { BaseLayoutProps } from 'fumadocs-ui/layouts/shared';

export const baseOptions: BaseLayoutProps = {
  nav: {
    title: (
      <div className="flex items-center gap-2">
        <img src="/goflow-logo.png" alt="GoFlow" className="h-8 w-auto" />
        <span className="font-bold">GoFlow</span>
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
      url: 'https://github.com/nuulab/goflow',
    },
  ],
};
