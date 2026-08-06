import { mount } from "@vue/test-utils";
import { describe, expect, it } from "vitest";
import AppIcon from "./AppIcon.vue";
import EmptyState from "./EmptyState.vue";

describe("shared interface primitives", () => {
  it("renders a consistent decorative SVG icon", () => {
    const wrapper = mount(AppIcon, { props: { name: "repository", size: 18 } });

    expect(wrapper.element.tagName).toBe("svg");
    expect(wrapper.attributes("width")).toBe("18");
    expect(wrapper.attributes("aria-hidden")).toBe("true");
    expect(wrapper.findAll("path").length).toBeGreaterThan(0);
  });

  it("renders the Help and external-link toolbar icons", () => {
    for (const name of ["help", "external-link"] as const) {
      const wrapper = mount(AppIcon, { props: { name } });
      expect(wrapper.findAll("path").length).toBeGreaterThan(0);
    }
  });

  it("renders reusable empty-state copy and optional actions", () => {
    const wrapper = mount(EmptyState, {
      props: {
        icon: "search",
        title: "没有搜索结果",
        description: "尝试缩短关键词。"
      },
      slots: {
        default: "<button>清空筛选</button>"
      }
    });

    expect(wrapper.text()).toContain("没有搜索结果");
    expect(wrapper.text()).toContain("尝试缩短关键词");
    expect(wrapper.get("button").text()).toBe("清空筛选");
  });
});
