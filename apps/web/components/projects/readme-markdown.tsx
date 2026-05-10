import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";

export function ReadmeMarkdown({ content }: { content: string }) {
  return (
    <div className="markdown">
      <ReactMarkdown
        remarkPlugins={[remarkGfm]}
        components={{
          a: ({ href, children }) => (
            <a href={href} rel="noreferrer" target="_blank">
              {children}
            </a>
          ),
        }}
      >
        {content}
      </ReactMarkdown>
    </div>
  );
}
