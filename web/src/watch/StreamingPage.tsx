import { useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import { api, type StreamPrefs } from "../api";
import Icon from "../components/Icon";
import { useDocumentTitle } from "../documentTitle";
import { Spinner } from "./parts";
import { useProfiles } from "./profiles";

// Streaming quality: how the profile on this device wants a version picked
// when it presses Play (as in apps like Nuvio). Kept for the profile on
// every device and app.

const DEFAULTS: StreamPrefs = {
  quality: "4k",
  size: "smart",
  remux: "avoid",
  hdr: "prefer",
  dolbyVision: "allow",
  efficient: true,
  lightSound: true,
};

type Choice<T extends string> = { value: T; label: string; hint: string };

const QUALITIES: Choice<StreamPrefs["quality"]>[] = [
  { value: "4k", label: "4K", hint: "The sharpest, on a 4K TV" },
  {
    value: "1080p",
    label: "1080p",
    hint: "Full HD, lighter on the connection",
  },
  {
    value: "720p",
    label: "720p",
    hint: "For a slow connection or a small screen",
  },
];

const SIZES: Choice<StreamPrefs["size"]>[] = [
  {
    value: "smart",
    label: "Smart",
    hint: "The size streaming services use: as sharp, starts fast, no stutter",
  },
  { value: "10", label: "Up to 10 GB", hint: "For a two-hour film" },
  { value: "20", label: "Up to 20 GB", hint: "For a two-hour film" },
  { value: "30", label: "Up to 30 GB", hint: "For a two-hour film" },
  { value: "50", label: "Up to 50 GB", hint: "For a two-hour film" },
  {
    value: "any",
    label: "Any size",
    hint: "The add-on’s own order, often the biggest file first",
  },
];

const DV: Choice<StreamPrefs["dolbyVision"]>[] = [
  { value: "prefer", label: "Prefer", hint: "Your TV shows Dolby Vision well" },
  { value: "allow", label: "Allow", hint: "Either way" },
  { value: "avoid", label: "Avoid", hint: "Colours look off on your TV" },
];

export default function StreamingPage() {
  const navigate = useNavigate();
  const { active } = useProfiles();
  const [prefs, setPrefs] = useState<StreamPrefs | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  useDocumentTitle("Streaming quality");

  useEffect(() => {
    api
      .streamPrefs()
      .then(setPrefs)
      .catch((e) => setError(e instanceof Error ? e.message : String(e)));
  }, []);

  if (!prefs)
    return error ? (
      <p className="wx-error" style={{ padding: 40 }}>
        {error}
      </p>
    ) : (
      <Spinner label="Loading…" />
    );
  const set = (p: Partial<StreamPrefs>) =>
    setPrefs((x) => (x ? { ...x, ...p } : x));

  async function save() {
    if (!prefs) return;
    setBusy(true);
    setError("");
    try {
      await api.saveStreamPrefs(prefs);
      navigate(-1);
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setBusy(false);
    }
  }

  function options<T extends string>(
    list: Choice<T>[],
    value: T,
    pick: (v: T) => void,
  ) {
    return (
      <div className="wx-theme-options">
        {list.map((c) => (
          <button
            key={c.value}
            className={`wx-theme-option ${value === c.value ? "on" : ""}`}
            onClick={() => pick(c.value)}
            aria-pressed={value === c.value}
          >
            <span>
              {c.label}
              <small>{c.hint}</small>
            </span>
          </button>
        ))}
      </div>
    );
  }

  function toggle(label: string, hint: string, on: boolean, flip: () => void) {
    return (
      <button
        className={`wx-theme-option ${on ? "on" : ""}`}
        onClick={flip}
        aria-pressed={on}
      >
        <Icon name={on ? "check" : "plus"} size={16} />
        <span>
          {label}
          <small>{hint}</small>
        </span>
      </button>
    );
  }

  return (
    <div className="wx-theme">
      <div className="wx-theme-head">
        <button
          className="wx-round"
          onClick={() => navigate(-1)}
          aria-label="Back"
        >
          <Icon name="arrow-left" size={20} />
        </button>
        <h1>
          Streaming quality{active ? <small> for {active.name}</small> : null}
        </h1>
      </div>
      <p className="wx-dim">
        How Cue picks a version when you press Play, on every device. Other
        versions are still there with Other version.
      </p>

      <section>
        <h2>Picture</h2>
        {options(QUALITIES, prefs.quality, (quality) => set({ quality }))}
      </section>

      <section>
        <h2>File size</h2>
        {options(SIZES, prefs.size, (size) => set({ size }))}
      </section>

      <section>
        <h2>Dolby Vision</h2>
        {options(DV, prefs.dolbyVision, (dolbyVision) => set({ dolbyVision }))}
      </section>

      <section>
        <h2>Prefer</h2>
        <div className="wx-theme-options">
          {toggle(
            "HDR",
            "Brighter, richer colours on a 4K HDR TV",
            prefs.hdr === "prefer",
            () => set({ hdr: prefs.hdr === "prefer" ? "any" : "prefer" }),
          )}
          {toggle(
            "Skip Remux",
            "Blu-ray copies of 40–80 GB: slow to start, little sharper",
            prefs.remux === "avoid",
            () => set({ remux: prefs.remux === "avoid" ? "allow" : "avoid" }),
          )}
          {toggle(
            "HEVC and AV1",
            "The same picture in about half the size",
            prefs.efficient,
            () => set({ efficient: !prefs.efficient }),
          )}
          {toggle(
            "Dolby Digital Plus and Atmos",
            "Over TrueHD and DTS-HD, which most TVs can’t send to speakers",
            prefs.lightSound,
            () => set({ lightSound: !prefs.lightSound }),
          )}
        </div>
      </section>

      {error && <p className="wx-error">{error}</p>}
      <div className="wx-actions">
        <button
          className="wx-btn play"
          disabled={busy}
          onClick={() => void save()}
        >
          {busy ? "Saving…" : "Save"}
        </button>
        <button className="wx-btn" onClick={() => setPrefs(DEFAULTS)}>
          Cue’s recommended
        </button>
      </div>
    </div>
  );
}
