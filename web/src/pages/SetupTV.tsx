import { useEffect, useState, type ReactNode } from 'react'
import BrandMark from '../components/BrandMark'
import { useDarkPage } from '../useDarkPage'
import { titleFor } from '../documentTitle'
import './setup-tv.css'

// Getting Cue on a TV, with a picture of each screen: what to find, what to
// press (lit), and the code to type. Open to anyone, signed in or not: it's
// what the family invite text links to, read on a phone beside the TV.

const CODE = '3575368'

type Device = 'fire' | 'google'

interface Step {
  title: string
  text: ReactNode
  art: ReactNode
}

function Screen({ children }: { children: ReactNode }) {
  return <div className="stv-screen">{children}</div>
}

// One menu row; lit = the one to press.
function Row({ children, lit, on }: { children: ReactNode; lit?: boolean; on?: boolean }) {
  return (
    <div className={`stv-row${lit ? ' lit' : ''}`}>
      <span>{children}</span>
      {on !== undefined && <i className={`stv-toggle${on ? ' on' : ''}`} />}
      {on === undefined && <b>›</b>}
    </div>
  )
}

function Path({ items }: { items: string[] }) {
  return (
    <p className="stv-path">
      {items.map((p, i) => (
        <span key={p}>
          {i > 0 && <em>›</em>}
          {p}
        </span>
      ))}
    </p>
  )
}

const searchArt = (store: string) => (
  <Screen>
    <div className="stv-search">
      <span className="stv-glass">⌕</span> Downloader
    </div>
    <div className="stv-results">
      <div className="stv-app lit">
        <span className="stv-dl">↓</span>
        <small>Downloader</small>
      </div>
      <div className="stv-app dim" />
      <div className="stv-app dim" />
    </div>
    <div className="stv-get">{store}</div>
  </Screen>
)

const codeArt = (
  <Screen>
    <div className="stv-dlbar">Downloader</div>
    <div className="stv-field">{CODE}</div>
    <div className="stv-keys">
      {['1', '2', '3', '4', '5', '6', '7', '8', '9', '0'].map((k) => (
        <span key={k}>{k}</span>
      ))}
      <span className="lit wide">Go</span>
    </div>
  </Screen>
)

const installArt = (
  <Screen>
    <div className="stv-dialog">
      <BrandMark className="stv-mark" />
      <p>Do you want to install this app?</p>
      <div className="stv-dialog-buttons">
        <span>Cancel</span>
        <span className="lit">Install</span>
      </div>
    </div>
  </Screen>
)

const signInArt = (
  <Screen>
    <div className="stv-signin">
      <div className="stv-qr" />
      <div>
        <BrandMark className="stv-mark" />
        <p>Scan with your phone's camera, then tap Sign in the TV</p>
      </div>
    </div>
  </Screen>
)

const steps: Record<Device, Step[]> = {
  fire: [
    {
      title: 'Get Downloader',
      text: (
        <>
          On the Fire TV's home screen, go to <b>Find</b> (the magnifying glass) and search <b>Downloader</b>. Pick the orange one and press <b>Get</b>.
        </>
      ),
      art: searchArt('Get'),
    },
    {
      title: 'Let Downloader install apps',
      text: (
        <>
          Go to <b>Settings</b> (the gear) and follow this path, then switch Downloader <b>On</b>.
          <Path items={['My Fire TV', 'Developer options', 'Install unknown apps', 'Downloader']} />
          <small>
            No Developer options? Go to My Fire TV › <b>About</b>, and press OK on the TV's name <b>7 times</b>. It appears.
          </small>
        </>
      ),
      art: (
        <Screen>
          <p className="stv-crumb">My Fire TV › Developer options › Install unknown apps</p>
          <Row>Silk Browser</Row>
          <Row lit on>
            Downloader
          </Row>
          <Row>Files</Row>
        </Screen>
      ),
    },
    {
      title: 'Type the code',
      text: (
        <>
          Open Downloader. Press <b>Allow</b> if it asks. In the box, type <b>{CODE}</b> and press <b>Go</b>.
        </>
      ),
      art: codeArt,
    },
    {
      title: 'Install Cue',
      text: (
        <>
          Cue downloads by itself. Press <b>Install</b>, then <b>Open</b>. (Downloader may offer to delete the file afterwards: say yes.)
        </>
      ),
      art: installArt,
    },
    {
      title: 'Sign in with your phone',
      text: <>Cue shows a code. Scan it with your phone's camera and tap Sign in the TV. That's it, and Cue updates itself from now on.</>,
      art: signInArt,
    },
  ],
  google: [
    {
      title: 'Get Downloader',
      text: (
        <>
          On the TV's home screen, search <b>Downloader</b> (or open the <b>Play Store</b> under Apps). Pick the orange one and press <b>Install</b>.
        </>
      ),
      art: searchArt('Install'),
    },
    {
      title: 'Turn on developer mode',
      text: (
        <>
          Go to <b>Settings</b> (the gear, top right) and follow this path, then press OK on <b>Android TV OS build</b> <b>7 times</b>, until it says you're a developer.
          <Path items={['System', 'About', 'Android TV OS build']} />
        </>
      ),
      art: (
        <Screen>
          <p className="stv-crumb">Settings › System › About</p>
          <Row>Model</Row>
          <Row>Android TV OS version</Row>
          <Row lit>Android TV OS build</Row>
        </Screen>
      ),
    },
    {
      title: 'Let Downloader install apps',
      text: (
        <>
          Back in <b>Settings</b>, follow this path and switch Downloader <b>On</b>.
          <Path items={['Apps', 'Security & restrictions', 'Unknown sources', 'Downloader']} />
        </>
      ),
      art: (
        <Screen>
          <p className="stv-crumb">Apps › Security & restrictions › Unknown sources</p>
          <Row>Chrome</Row>
          <Row lit on>
            Downloader
          </Row>
          <Row>Files</Row>
        </Screen>
      ),
    },
    {
      title: 'Type the code',
      text: (
        <>
          Open Downloader. Press <b>Allow</b> if it asks. In the box, type <b>{CODE}</b> and press <b>Go</b>.
        </>
      ),
      art: codeArt,
    },
    {
      title: 'Install Cue, then sign in',
      text: <>Press <b>Install</b>, then <b>Open</b>. Cue shows a code: scan it with your phone's camera and tap Sign in the TV. Cue updates itself from now on.</>,
      art: installArt,
    },
  ],
}

export default function SetupTV() {
  useDarkPage(true)
  const [device, setDevice] = useState<Device>(() => (new URLSearchParams(location.search).get('device') === 'google' ? 'google' : 'fire'))
  useEffect(() => {
    document.title = titleFor('Get Cue on your TV')
  }, [])
  return (
    <main className="stv">
      <header>
        <BrandMark className="stv-logo" />
        <h1>Get Cue on your TV</h1>
        <p>A few minutes, once. Keep this open on your phone beside the TV.</p>
      </header>
      <div className="stv-tabs" role="tablist" aria-label="Your TV">
        {(
          [
            ['fire', 'Fire TV'],
            ['google', 'Google TV'],
          ] as const
        ).map(([id, name]) => (
          <button key={id} role="tab" aria-selected={device === id} className={device === id ? 'on' : ''} onClick={() => setDevice(id)}>
            {name}
          </button>
        ))}
      </div>
      <ol className="stv-steps">
        {steps[device].map((s, i) => (
          <li key={s.title}>
            <div className="stv-art" aria-hidden="true">
              {s.art}
            </div>
            <div className="stv-words">
              <span className="stv-n">{i + 1}</span>
              <h2>{s.title}</h2>
              <div className="stv-text">{s.text}</div>
            </div>
          </li>
        ))}
      </ol>
      <p className="stv-foot">
        Stuck? Send a photo of the TV to whoever invited you. On a phone or laptop, just go to <b>{location.host}</b>.
      </p>
    </main>
  )
}
