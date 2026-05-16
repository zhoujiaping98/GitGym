import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import App from "../App";

describe("App", () => {
  it("renders the GitHub login link", () => {
    render(<App />);

    const loginLink = screen.getByRole("link", {
      name: "Continue with GitHub",
    });

    expect(loginLink).toBeInTheDocument();
    expect(loginLink).toHaveAccessibleName("Continue with GitHub");
    expect(loginLink).toHaveAttribute("href", "/api/v1/auth/github/login");
    expect(
      screen.getByText(/safe trial repository, real git behavior, and a resettable environment/i),
    ).toBeInTheDocument();
    expect(screen.getByText("Template: Standard")).toBeInTheDocument();
  });
});
