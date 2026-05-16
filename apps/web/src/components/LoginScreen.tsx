import type { ReactNode } from "react";

type LoginScreenProps = {
  preview: ReactNode;
};

export function LoginScreen({ preview }: LoginScreenProps) {
  return (
    <main className="login-screen">
      <section className="login-copy">
        <p className="eyebrow">Browser workbench for real Git practice</p>
        <h1>Run the risky Git sequence in here before it touches local work.</h1>
        <p className="lede">
          GitGym gives you a safe trial repository, real Git behavior, and a
          resettable environment when the command sequence goes sideways.
        </p>
        <div className="promise-list" aria-label="Product promises">
          <span>Safe trial</span>
          <span>Real Git</span>
          <span>Reset in seconds</span>
        </div>
        <a className="primary-button" href="/api/v1/auth/github/login">
          Continue with GitHub
        </a>
        <p className="supporting-note">
          GitHub login unlocks a disposable browser session. No local repository
          changes. No setup steps before practice.
        </p>
      </section>
      <section className="login-preview" aria-label="Workbench preview">
        <div className="preview-caption">
          <span className="preview-label">Workbench preview</span>
          <p>The terminal stays primary. Repository state and session context stay visible.</p>
        </div>
        <div className="preview-frame" aria-hidden="true">
          {preview}
        </div>
      </section>
    </main>
  );
}
