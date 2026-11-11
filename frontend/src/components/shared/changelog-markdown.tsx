import { ArrowUpFromDot, Minus, Plus, SparklesIcon, TriangleAlert, Wrench } from "lucide-react";

import { FaGithub } from "react-icons/fa";
import ReactMarkdown from "react-markdown";

import { cn } from "@/lib/cn";

interface ChangelogMarkdownProps {
  currentVersion?: string | null;
  children: string;
}

export function ChangelogMarkdown({ currentVersion, children }: ChangelogMarkdownProps) {
  return (
    <div>
      <ReactMarkdown
        components={{
          a: ({ node: _node, href, children, ...props }) => {
            const isGithub = href?.includes("github.com");
            return (
              <a
                href={href}
                target="_blank"
                rel="noopener noreferrer"
                className="inline-flex items-center gap-1 underline text-primary active:scale-95 hover:brightness-120"
                {...props}
              >
                {children}
                {isGithub && <FaGithub className="inline-block w-4 h-4" />}
              </a>
            );
          },
          h2: ({ node: _node, ...props }) => {
            const headingText = Array.isArray(props.children)
              ? props.children.map(String).join("")
              : String(props.children ?? "");
            const match = /(\[([^\]]+)\])\s*-\s*(\d{4}-\d{2}-\d{2})/.exec(headingText);
            const version = match ? match[2] : null;
            const normalize = (v: string) => v.replace(/^v/, "").replace(/-.*$/, "");
            if (match && version && currentVersion && normalize(version) === normalize(currentVersion)) {
              return (
                <>
                  <div className="my-8 flex items-center gap-3">
                    <hr className="flex-grow border-yellow-400 border-t-2" />
                    <span className="bg-yellow-100 text-yellow-800 px-3 py-1 rounded font-semibold text-sm shadow border border-yellow-300">
                      You are here: {currentVersion}
                    </span>
                    <hr className="flex-grow border-yellow-400 border-t-2" />
                  </div>
                  <div className="flex items-center gap-2 mt-1 mb-2">
                    <span className="text-primary font-bold text-lg">{match[1]}</span>
                    <span className="text-muted-foreground text-sm">{match[3]}</span>
                  </div>
                </>
              );
            }
            if (match) {
              return (
                <div className="flex items-center gap-2 mt-1 mb-2">
                  <span className="text-primary font-bold text-lg">{match[1]}</span>
                  <span className="text-muted-foreground text-sm">{match[3]}</span>
                </div>
              );
            }
            return <h2 className="text-xl font-semibold mt-6 mb-2 border-b pb-1" {...props} />;
          },
          h3: ({ ...props }) => {
            const text = String(props.children);
            if (/breaking/i.test(text)) {
              return (
                <h3 className="text-lg font-semibold mb-1 text-red-600 flex items-center gap-2">
                  <TriangleAlert className="w-5 h-5" />
                  {props.children}
                </h3>
              );
            }
            if (/added/i.test(text)) {
              return (
                <h3 className="text-lg font-semibold mb-1 text-green-600 flex items-center gap-2">
                  <Plus className="w-5 h-5" />
                  {props.children}
                </h3>
              );
            }
            if (/fixed/i.test(text)) {
              return (
                <h3 className="text-lg font-semibold mb-1 text-yellow-600 flex items-center gap-2">
                  <Wrench className="w-5 h-5" />
                  {props.children}
                </h3>
              );
            }

            if (/ui tweaks/i.test(text)) {
              return (
                <h3 className="text-lg font-semibold mb-1 text-blue-600 flex items-center gap-2">
                  <SparklesIcon className="w-5 h-5" />
                  {props.children}
                </h3>
              );
            }

            if (/removed/i.test(text)) {
              return (
                <h3 className="text-lg font-semibold mb-1 text-orange-600 flex items-center gap-2">
                  <Minus className="w-5 h-5" />
                  {props.children}
                </h3>
              );
            }

            if (/updated/i.test(text)) {
              return (
                <h3 className="text-lg font-semibold mb-1 text-purple-600 flex items-center gap-2">
                  <ArrowUpFromDot className="w-5 h-5" />
                  {props.children}
                </h3>
              );
            }
            return <h3 className="text-lg font-semibold mt-4 mb-1">{props.children}</h3>;
          },
          ul: ({ node: _node, ...props }) => <ul className="text-md list-disc ml-6 mb-2" {...props} />,
          ol: ({ node: _node, ...props }) => <ol className="text-md list-decimal ml-6 mb-2" {...props} />,
          li: ({ node: _node, ...props }) => <li className="text-md mb-1" {...props} />,
          // react-markdown no longer says if code is inline, so code blocks are styled by pre
          pre: ({ node: _node, ...props }) => (
            <pre
              className="bg-zinc-100 dark:bg-zinc-800 p-3 rounded mb-4 overflow-x-auto [&>code]:p-0 [&>code]:text-base"
              {...props}
            />
          ),
          code: ({ node: _node, className, ...props }) => (
            <code className={cn("bg-zinc-100 dark:bg-zinc-800 px-1 rounded text-sm", className)} {...props} />
          ),
          blockquote: ({ node: _node, ...props }) => (
            <blockquote className="border-l-4 border-primary pl-4 italic text-muted-foreground my-4" {...props} />
          ),
          p: ({ node: _node, ...props }) => <p className="mb-2" {...props} />,
          hr: () => <hr className="my-6 border-muted-foreground/20" />,
        }}
      >
        {children}
      </ReactMarkdown>
    </div>
  );
}
