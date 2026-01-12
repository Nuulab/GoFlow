'use client';

import { useEffect, useId, useState } from 'react';

export function Mermaid({ chart }: { chart: string }) {
  const [mounted, setMounted] = useState(false);
  
  useEffect(() => {
    setMounted(true);
  }, []);
  
  if (!mounted) return null;
  
  return <MermaidContent chart={chart} />;
}

const cache = new Map<string, Promise<string>>();

function MermaidContent({ chart }: { chart: string }) {
  const id = useId().replace(/:/g, '');
  const [svg, setSvg] = useState<string>('');
  const [error, setError] = useState<string>('');
  
  useEffect(() => {
    const renderDiagram = async () => {
      try {
        // Check cache first
        const cacheKey = chart;
        let cached = cache.get(cacheKey);
        
        if (!cached) {
          const mermaid = (await import('mermaid')).default;
          
          mermaid.initialize({
            startOnLoad: false,
            securityLevel: 'loose',
            fontFamily: 'inherit',
            theme: document.documentElement.classList.contains('dark') ? 'dark' : 'default',
          });
          
          cached = mermaid.render(`mermaid-${id}`, chart.replaceAll('\\n', '\n')).then(r => r.svg);
          cache.set(cacheKey, cached);
        }
        
        const result = await cached;
        setSvg(result);
      } catch (e) {
        setError(e instanceof Error ? e.message : 'Failed to render diagram');
      }
    };
    
    renderDiagram();
  }, [chart, id]);
  
  if (error) {
    return (
      <div className="p-4 border border-red-500/50 rounded-lg bg-red-500/10 text-red-400 text-sm">
        Mermaid Error: {error}
      </div>
    );
  }
  
  if (!svg) {
    return (
      <div className="p-4 border border-fd-border rounded-lg bg-fd-muted/50 text-fd-muted-foreground text-sm animate-pulse">
        Loading diagram...
      </div>
    );
  }
  
  return (
    <div 
      className="my-6 p-4 bg-fd-card border border-fd-border rounded-xl overflow-x-auto"
      dangerouslySetInnerHTML={{ __html: svg }} 
    />
  );
}
