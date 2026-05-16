import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import App from "../App";

describe("App", () => {
  it("renders the login call to action", () => {
    render(<App />);

    expect(screen.getByText("Continue with GitHub")).toBeInTheDocument();
    expect(
      screen.getByText(/safe trial repository, real git behavior, and a resettable environment/i),
    ).toBeInTheDocument();
    expect(screen.getByText("Template: Standard")).toBeInTheDocument();
  });
});
