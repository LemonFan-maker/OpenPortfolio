import { blogURL } from "@/lib/site-config";

export default function AboutPage() {
  return (
    <section className="section">
      <div className="container max-w-3xl">
        <h1 className="text-4xl font-black">关于</h1>
        <p className="mt-5 text-lg leading-8 text-[var(--muted)]">
          这里用于展示个人技术方向、开源项目和博客入口。配置 GitHub 用户名后，站点会自动获取公开仓库并生成项目展示数据。
        </p>
        <a className="btn btn-primary mt-8" href={blogURL}>
          访问博客
        </a>
      </div>
    </section>
  );
}
