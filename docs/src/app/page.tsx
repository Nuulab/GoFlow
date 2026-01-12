import Link from 'next/link';

export default function HomePage() {
  return (
    <main className="flex h-screen flex-col justify-center text-center">
      <h1 className="mb-4 text-4xl font-bold">GoFlow</h1>
      <p className="text-fd-muted-foreground mb-8">
        Production-ready AI Agent & Workflow Framework for Go
      </p>
      <div className="flex justify-center gap-4">
        <Link
          href="/docs"
          className="rounded-lg bg-fd-primary px-6 py-3 text-fd-primary-foreground font-medium hover:bg-fd-primary/90 transition-colors"
        >
          Read Docs
        </Link>
        <Link
          href="https://github.com/nuulab/goflow"
          className="rounded-lg border border-fd-border px-6 py-3 font-medium hover:bg-fd-accent transition-colors"
        >
          GitHub
        </Link>
      </div>
    </main>
  );
}
