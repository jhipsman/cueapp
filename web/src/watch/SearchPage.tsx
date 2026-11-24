import { useEffect, useState } from "react";
import { useNavigate, useSearchParams } from "react-router-dom";
import { api, type LiveHit, type WatchSearchAnswer } from "../api";
import { useDocumentTitle } from "../documentTitle";
import ChannelLogo from "./ChannelLogo";
import { liveHref } from "./LivePage";
import { Card, Row } from "./parts";
import { useProfiles } from "./profiles";
import { useReminders } from "./reminders";
import Icon from "../components/Icon";
import { GenreTiles, Pills, useGenre, type KindFilter } from "./browse";

const hhmm = (s: string) =>
  new Date(s).toLocaleTimeString([], { hour: "numeric", minute: "2-digit" });

// when says when a show is on: "Ends 8:45 PM" while it's on, "7:30 PM"
// today, "Tomorrow 1:00 PM", or the weekday.
function when(h: LiveHit): string {
  if (!h.show) return "";
  if (h.onNow) return `Ends ${hhmm(h.show.stop)}`;
  const start = new Date(h.show.start);
  const today = new Date();
  const days = Math.round(
    (new Date(start).setHours(0, 0, 0, 0) -
      new Date(today).setHours(0, 0, 0, 0)) /
      86_400_000,
  );
  const day =
    days === 0
      ? "Today"
      : days === 1
        ? "Tomorrow"
        : start.toLocaleDateString([], { weekday: "long" });
  return `${day} ${hhmm(h.show.start)}`;
}

// Search: every movie and show TMDB knows, best match first, and with Live
// TV set up, games and shows on now and coming up, and channels by name.
// The search box is in the top bar.
export default function SearchPage() {
  const [params, setParams] = useSearchParams();
  const genre = params.get("genre") ?? "";
  const kind = (params.get("kind") as KindFilter | null) ?? "all";
  const setKind = (k: KindFilter) => {
    const next = new URLSearchParams(params);
    if (k === "all") next.delete("kind");
    else next.set("kind", k);
    setParams(next, { replace: true });
  };
  const byGenre = useGenre(genre, kind);
  const navigate = useNavigate();
  const liveTV = useProfiles().data?.liveTV ?? false;
  const reminders = useReminders();
  const q = params.get("q") ?? "";
  const [results, setResults] = useState<{
    q: string;
    answer: WatchSearchAnswer;
  } | null>(null);
  const [live, setLive] = useState<{ q: string; hits: LiveHit[] } | null>(null);
  const [error, setError] = useState("");
  // Follows: words that get a reminder whenever a show with them is on.
  const [follows, setFollows] = useState<string[]>([]);
  useEffect(() => {
    if (!liveTV) return;
    api
      .liveFollows()
      .then(setFollows)
      .catch(() => undefined);
  }, [liveTV]);
  const following = follows.some((f) => f.toLowerCase() === q.trim().toLowerCase());
  async function toggleFollow(phrase: string, on: boolean) {
    try {
      if (on) await api.followLive(phrase);
      else await api.unfollowLive(phrase);
      setFollows(await api.liveFollows());
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    }
  }
  useDocumentTitle(q ? `Search: ${q}` : genre ? genre : "Search");

  useEffect(() => {
    if (!q.trim()) return;
    let current = true;
    api
      .watchSearchRows(q)
      .then((answer) => {
        if (current) {
          setResults({ q, answer });
          setError("");
        }
      })
      .catch(
        (e) => current && setError(e instanceof Error ? e.message : String(e)),
      );
    if (liveTV) {
      api
        .liveSearch(q)
        .then((r) => current && setLive({ q, hits: r.hits }))
        .catch(() => current && setLive({ q, hits: [] }));
    }
    return () => {
      current = false;
    };
  }, [q, liveTV]);

  // Each kind of answer is a row (a person's films, what the search
  // describes, a topic, titles named like it); the pills narrow them all.
  const answer = results?.q === q ? results.answer : null;
  const rows = (answer?.sections ?? [])
    .map((sec) => ({ ...sec, items: kind === "all" ? sec.items : sec.items.filter((c) => c.kind === kind) }))
    .filter((sec) => sec.items.length > 0);
  const found = (answer?.sections ?? []).flatMap((sec) => sec.items).filter((c, i, all) => all.findIndex((o) => o.kind === c.kind && o.tmdbId === c.tmdbId) === i);
  const items = kind === "all" ? found : found.filter((c) => c.kind === kind);
  const hits = live?.q === q ? live.hits : [];
  const shows = hits.filter((h) => h.show);
  const onNow = shows.filter((h) => h.onNow);
  const later = shows.filter((h) => !h.onNow);
  const channels = hits.filter((h) => !h.show);
  const done = results?.q === q && (!liveTV || live?.q === q);

  const LiveCard = ({ h }: { h: LiveHit }) => (
    <div className="wx-live-hit-wrap">
      <button
        className={`wx-live-hit${h.onNow ? " now" : ""}`}
        onClick={() => navigate(liveHref(h.channel.id, "all"))}
      >
        <span className="wx-live-hit-logo">
          <ChannelLogo name={h.channel.name} src={h.channel.logo} />
        </span>
        <span className="wx-live-hit-body">
          {h.onNow && (
            <span className="wx-live-hit-badge">
              <i /> LIVE
            </span>
          )}
          <b>{h.show?.title ?? h.channel.name}</b>
          <small>
            {h.show
              ? `${h.channel.name} · ${when(h)}`
              : h.channel.now
                ? `Now: ${h.channel.now.title}`
                : `Channel ${h.channel.num || ""}`}
          </small>
        </span>
      </button>
      {h.show && !h.onNow && (
        <button
          className={`wx-live-hit-remind${reminders.has(h.channel.id, h.show.start) ? " on" : ""}`}
          onClick={() => h.show && void reminders.toggle(h.channel.id, h.show)}
          aria-label={
            reminders.has(h.channel.id, h.show.start)
              ? "Reminder set: remove it"
              : "Remind me when it starts"
          }
          title={
            reminders.has(h.channel.id, h.show.start)
              ? "Reminder set"
              : "Remind me"
          }
        >
          <Icon
            name={reminders.has(h.channel.id, h.show.start) ? "check" : "clock"}
            size={18}
          />
        </button>
      )}
    </div>
  );

  // Nothing typed: the genres, or a genre's titles.
  if (!q.trim()) {
    return (
      <div className="wx-page wx-search-home">
        <div className="wx-search-top">
          {genre ? (
            <h1>
              <button className="wx-linkish" onClick={() => navigate(kind === "all" ? "/watch/search" : `/watch/search?kind=${kind}`)} aria-label="Back to genres">
                <Icon name="chevron-left" size={22} />
              </button>
              {genre}
            </h1>
          ) : (
            <h1>Search</h1>
          )}
          <Pills value={kind} onChange={setKind} />
        </div>
        {!genre && <p className="wx-dim">Type a title, an actor or a channel above, or start from a genre.</p>}
        {!genre && <GenreTiles kind={kind} />}
        {genre && byGenre.error && <p className="wx-error">{byGenre.error}</p>}
        {genre && byGenre.items === null && <div className="wx-grid">{Array.from({ length: 12 }, (_, i) => <div key={i} className="wx-card wx-skeleton" style={{ aspectRatio: "2 / 3" }} />)}</div>}
        {genre && byGenre.items && (
          <div className="wx-grid">
            {byGenre.items.map((c) => (
              <Card key={`${c.kind}-${c.tmdbId}`} card={c} />
            ))}
          </div>
        )}
      </div>
    );
  }

  return (
    <div className="wx-page">
      {error && <p className="wx-error">{error}</p>}
      {results?.q === q && found.length > 0 && (
        <div className="wx-search-top">
          <h1>{answer?.didYouMean ? <>Showing results for “{answer.didYouMean}”</> : <>Results for “{q}”</>}</h1>
          <Pills
            value={kind}
            onChange={setKind}
            counts={{ all: found.length, movie: found.filter((c) => c.kind === "movie").length, tv: found.filter((c) => c.kind === "tv").length }}
          />
        </div>
      )}
      {done && items.length === 0 && hits.length === 0 && (
        <p className="wx-dim">Nothing found for “{q}”{kind !== "all" ? ` in ${kind === "movie" ? "movies" : "shows"}` : ""}.</p>
      )}

      {liveTV && q.trim() && (shows.length > 0 || following) && (
        <div className="wx-follow">
          <Icon name="clock" size={18} />
          <span>
            {following ? (
              <>
                Following <b>“{q.trim()}”</b>: you get a reminder whenever it's on Live TV.
              </>
            ) : (
              <>
                Get a reminder whenever <b>“{q.trim()}”</b> is on Live TV.
              </>
            )}
          </span>
          <button className={`wx-btn small${following ? "" : " play"}`} onClick={() => void toggleFollow(q.trim(), !following)}>
            {following ? "Unfollow" : "Follow"}
          </button>
        </div>
      )}
      {liveTV && follows.length > 0 && (
        <div className="wx-follows">
          <small>Following:</small>
          {follows.map((f) => (
            <span key={f} className="wx-follow-chip">
              <button onClick={() => navigate(`/watch/search?q=${encodeURIComponent(f)}`)}>{f}</button>
              <button onClick={() => void toggleFollow(f, false)} aria-label={`Stop following ${f}`}>
                <Icon name="x" size={12} />
              </button>
            </span>
          ))}
        </div>
      )}

      {onNow.length > 0 && (
        <section className="wx-search-live">
          <h2>On now</h2>
          <div className="wx-live-hits">
            {onNow.map((h) => (
              <LiveCard key={`${h.channel.id}-${h.show?.start}`} h={h} />
            ))}
          </div>
        </section>
      )}

      {rows.length === 1 && (
        <section>
          {hits.length > 0 && <h2>Movies and shows</h2>}
          <div className="wx-grid">
            {rows[0].items.map((c) => (
              <Card key={`${c.kind}-${c.tmdbId}`} card={c} />
            ))}
          </div>
        </section>
      )}
      {rows.length > 1 && rows.map((sec) => <Row key={sec.key} rowKey={`search-${sec.key}`} title={sec.title} items={sec.items} />)}

      {later.length > 0 && (
        <section className="wx-search-live">
          <h2>Coming up on Live TV</h2>
          <div className="wx-live-hits">
            {later.map((h) => (
              <LiveCard key={`${h.channel.id}-${h.show?.start}`} h={h} />
            ))}
          </div>
        </section>
      )}

      {channels.length > 0 && (
        <section className="wx-search-live">
          <h2>Channels</h2>
          <div className="wx-live-hits">
            {channels.map((h) => (
              <LiveCard key={h.channel.id} h={h} />
            ))}
          </div>
        </section>
      )}
    </div>
  );
}
