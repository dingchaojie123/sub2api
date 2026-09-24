import { describe, expect, it } from "vitest";

import {
  getDefaultImagePreviewPrice,
  getDefaultVideoPreviewPrice,
  getImagePricePlaceholder,
  getVideoPricePlaceholder,
  imagePricingPlatforms,
  imagePricingI18nKey,
  isPPVideoPricingPlatform,
  usesGroupVideoPriceConfig,
  supportsImagePricingPlatform,
  supportsVideoPricingPlatform,
  videoPricingI18nKey,
} from "../groupsImagePricing";

describe("groups image pricing platform support", () => {
  it("includes Grok image groups", () => {
    expect(supportsImagePricingPlatform("grok")).toBe(true);
    expect(imagePricingPlatforms.has("grok")).toBe(true);
  });

  it("includes Doubao image groups", () => {
    expect(supportsImagePricingPlatform("doubao")).toBe(true);
    expect(imagePricingPlatforms.has("doubao")).toBe(true);
  });

  it("includes Midjourney image groups", () => {
    expect(supportsImagePricingPlatform("midjourney")).toBe(true);
    expect(imagePricingPlatforms.has("midjourney")).toBe(true);
  });

  it("enables video pricing controls for Grok and PP video platforms", () => {
    expect(supportsVideoPricingPlatform("grok")).toBe(true);
    expect(supportsVideoPricingPlatform("kling")).toBe(true);
    expect(supportsVideoPricingPlatform("happyhourse")).toBe(true);
    expect(supportsVideoPricingPlatform("seedance")).toBe(true);
    expect(supportsVideoPricingPlatform("bytedance")).toBe(true);
    expect(supportsVideoPricingPlatform("wan3")).toBe(true);
    expect(supportsVideoPricingPlatform("minimax-h3")).toBe(true);
    expect(supportsVideoPricingPlatform("pixverse-v6")).toBe(true);
    expect(supportsVideoPricingPlatform("grok-imagine-video")).toBe(true);
    expect(supportsVideoPricingPlatform("kuaishou")).toBe(true);
    expect(supportsVideoPricingPlatform("openai")).toBe(false);
  });

  it("uses the existing group video price card for per-second PP platforms", () => {
    expect(isPPVideoPricingPlatform("kling")).toBe(true);
    expect(isPPVideoPricingPlatform("happyhourse")).toBe(true);
    expect(isPPVideoPricingPlatform("seedance")).toBe(true);
    expect(isPPVideoPricingPlatform("bytedance")).toBe(true);
    expect(isPPVideoPricingPlatform("wan3")).toBe(true);
    expect(isPPVideoPricingPlatform("minimax-h3")).toBe(true);
    expect(isPPVideoPricingPlatform("pixverse-v6")).toBe(true);
    expect(isPPVideoPricingPlatform("grok-imagine-video")).toBe(true);
    expect(isPPVideoPricingPlatform("kuaishou")).toBe(true);
    expect(isPPVideoPricingPlatform("grok")).toBe(false);
    expect(usesGroupVideoPriceConfig("kling")).toBe(true);
    expect(usesGroupVideoPriceConfig("happyhourse")).toBe(true);
    expect(usesGroupVideoPriceConfig("seedance")).toBe(true);
    expect(usesGroupVideoPriceConfig("bytedance")).toBe(true);
    expect(usesGroupVideoPriceConfig("wan3")).toBe(true);
    expect(usesGroupVideoPriceConfig("minimax-h3")).toBe(true);
    expect(usesGroupVideoPriceConfig("pixverse-v6")).toBe(true);
    expect(usesGroupVideoPriceConfig("grok-imagine-video")).toBe(true);
    expect(usesGroupVideoPriceConfig("kuaishou")).toBe(true);
  });

  it("keeps non-media group platforms out of the image pricing controls", () => {
    expect(supportsImagePricingPlatform("anthropic")).toBe(false);
  });

  it("keeps image and video pricing copy separate", () => {
    expect(imagePricingI18nKey("grok", "title")).toBe(
      "admin.groups.imagePricing.title",
    );
    expect(videoPricingI18nKey("title")).toBe("admin.groups.videoPricing.title");
  });

  it("uses Grok media defaults instead of generic image fallback placeholders", () => {
    expect(getImagePricePlaceholder("grok", "image_price_1k")).toBe("0.02");
    expect(getImagePricePlaceholder("grok", "image_price_2k")).toBe("0.02");
    // 视频 placeholder 为每秒单价：480p/720p 取 grok-imagine-video 官方每秒价，
    // 1080p 仅 video-1.5 支持、取 1.5 每秒价。
    expect(getVideoPricePlaceholder("grok", "video_price_480p")).toBe("0.05");
    expect(getVideoPricePlaceholder("grok", "video_price_720p")).toBe("0.07");
    expect(getVideoPricePlaceholder("grok", "video_price_1080p")).toBe("0.25");
  });

  it("keeps non-Grok image placeholders on the generic image card", () => {
    expect(getImagePricePlaceholder("openai", "image_price_1k")).toBe("0.134");
    expect(getDefaultImagePreviewPrice("openai", "image_price_2k")).toBe(0.201);
    expect(getDefaultVideoPreviewPrice("openai", "video_price_480p")).toBeNull();
  });
});
