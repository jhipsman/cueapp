import { useEffect, useRef, useState } from 'react'
import * as THREE from 'three'
import { GLTFLoader } from 'three/examples/jsm/loaders/GLTFLoader.js'
import { RoomEnvironment } from 'three/examples/jsm/environments/RoomEnvironment.js'

// What a phone's screen shows when it isn't a clip: drawn each frame onto
// a canvas the screen's size (t is seconds since it started).
type ScreenPainter = (ctx: CanvasRenderingContext2D, w: number, h: number, t: number) => void

// PhoneModel: a real 3D phone, with Cue's phone clip (or a drawn screen)
// playing on its screen, kept to the screen's own proportions.
// It turns gently toward the pointer and floats a little. Until the model
// has loaded (or where there's no WebGL, or the viewer asks for less
// motion) the drawn phone underneath shows instead. It draws only while
// it's on screen.
export default function PhoneModel({ model, clip, poster, searchPoster, turn = -0.28 }: {
  model: string
  clip?: string
  poster?: string
  searchPoster?: string // no clip: the search page, finding this film
  turn?: number
}) {
  const box = useRef<HTMLDivElement>(null)
  const [ready, setReady] = useState(false)

  useEffect(() => {
    const el = box.current
    if (!el || matchMedia('(prefers-reduced-motion: reduce)').matches) return
    let renderer: THREE.WebGLRenderer
    try {
      renderer = new THREE.WebGLRenderer({ antialias: true, alpha: true, powerPreference: 'low-power' })
    } catch {
      return // no WebGL: the drawn phone stays
    }
    renderer.setPixelRatio(Math.min(window.devicePixelRatio, 2))
    renderer.outputColorSpace = THREE.SRGBColorSpace
    renderer.toneMapping = THREE.ACESFilmicToneMapping
    renderer.toneMappingExposure = 1.05
    el.appendChild(renderer.domElement)

    const scene = new THREE.Scene()
    const pmrem = new THREE.PMREMGenerator(renderer)
    const env = pmrem.fromScene(new RoomEnvironment(), 0.04).texture
    scene.environment = env
    const camera = new THREE.PerspectiveCamera(22, 1, 0.01, 100)

    // The screen: a canvas the screen's shape, with the clip drawn into it
    // cropped to fill (as a picture would), or the painter's drawing.
    const video = document.createElement('video')
    if (clip) {
      Object.assign(video, { src: clip, poster: poster ?? '', muted: true, loop: true, playsInline: true })
      video.setAttribute('playsinline', '')
    }
    const canvas = document.createElement('canvas')
    canvas.width = 720
    canvas.height = 1560
    const ctx = canvas.getContext('2d')!
    const paint: ScreenPainter | undefined = searchPoster ? searchScreen(searchPoster) : undefined
    const screen = new THREE.CanvasTexture(canvas)
    screen.colorSpace = THREE.SRGBColorSpace
    screen.flipY = false
    screen.anisotropy = 4
    const paintScreen = (t: number) => {
      const w = canvas.width
      const h = canvas.height
      if (clip && video.readyState >= 2 && video.videoWidth) {
        const scale = Math.max(w / video.videoWidth, h / video.videoHeight)
        const dw = video.videoWidth * scale
        const dh = video.videoHeight * scale
        ctx.fillStyle = '#000'
        ctx.fillRect(0, 0, w, h)
        ctx.drawImage(video, (w - dw) * 0.12, 0, dw, dh)
      } else if (paint) {
        paint(ctx, w, h, t)
      } else {
        ctx.fillStyle = '#000'
        ctx.fillRect(0, 0, w, h)
      }
      screen.needsUpdate = true
    }

    const rig = new THREE.Group()
    scene.add(rig)
    let stopped = false
    new GLTFLoader().load(
      model,
      (gltf) => {
        if (stopped) return
        const phone = gltf.scene
        phone.traverse((o) => {
          const mesh = o as THREE.Mesh
          if (mesh.isMesh && mesh.name === 'Front_Screen') {
            mesh.material = new THREE.MeshBasicMaterial({ map: screen, toneMapped: false })
            // The canvas takes the screen's own shape.
            mesh.geometry.computeBoundingBox()
            const b = mesh.geometry.boundingBox!.getSize(new THREE.Vector3())
            const across = Math.min(b.x, b.y)
            const tall = Math.max(b.x, b.y)
            if (across > 0) canvas.height = Math.round((canvas.width * tall) / across)
          }
        })
        // Centred, standing up, sized to fill the view.
        const size = new THREE.Box3().setFromObject(phone)
        const centre = size.getCenter(new THREE.Vector3())
        const dims = size.getSize(new THREE.Vector3())
        phone.position.sub(centre)
        rig.add(phone)
        const height = Math.max(dims.x, dims.y, dims.z)
        camera.position.set(0, 0, (height / 2 / Math.tan(THREE.MathUtils.degToRad(camera.fov / 2))) * 1.08)
        camera.lookAt(0, 0, 0)
        resize()
        setReady(true)
      },
      undefined,
      () => undefined, // couldn't load: the drawn phone stays
    )

    function resize() {
      const w = el!.clientWidth
      const h = el!.clientHeight
      if (!w || !h) return
      renderer.setSize(w, h, false)
      camera.aspect = w / h
      camera.updateProjectionMatrix()
    }
    const ro = new ResizeObserver(resize)
    ro.observe(el)

    // Turns toward the pointer, eased; floats when left alone.
    const aim = { x: 0, y: 0 }
    const now = { x: 0, y: 0 }
    const onMove = (e: PointerEvent) => {
      aim.y = (e.clientX / window.innerWidth - 0.5) * 0.5
      aim.x = (e.clientY / window.innerHeight - 0.5) * 0.25
    }
    window.addEventListener('pointermove', onMove, { passive: true })

    let frame = 0
    let visible = false
    const t0 = performance.now()
    const draw = () => {
      frame = requestAnimationFrame(draw)
      const t = (performance.now() - t0) / 1000
      now.x += (aim.x - now.x) * 0.06
      now.y += (aim.y - now.y) * 0.06
      rig.rotation.set(now.x + Math.sin(t * 0.6) * 0.02, turn + now.y + Math.sin(t * 0.4) * 0.05, 0.05)
      rig.position.y = Math.sin(t * 0.8) * 0.002
      paintScreen(t)
      renderer.render(scene, camera)
    }
    const io = new IntersectionObserver(([e]) => {
      visible = e.isIntersecting
      if (visible && !frame) {
        if (clip) void video.play().catch(() => undefined)
        draw()
      } else if (!visible && frame) {
        cancelAnimationFrame(frame)
        frame = 0
        video.pause()
      }
    })
    io.observe(el)

    return () => {
      stopped = true
      io.disconnect()
      ro.disconnect()
      window.removeEventListener('pointermove', onMove)
      cancelAnimationFrame(frame)
      video.pause()
      video.removeAttribute('src')
      screen.dispose()
      env.dispose()
      pmrem.dispose()
      renderer.dispose()
      renderer.domElement.remove()
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [model, clip, poster, searchPoster, turn])

  return <div className={`lp-phone3d${ready ? ' on' : ''}`} ref={box} aria-hidden="true" />
}

// The phone's search page, searching the TV: "Dune" is typed, then the
// film appears. Loops every six seconds.
function searchScreen(posterUrl: string): ScreenPainter {
  const poster = new Image()
  poster.src = posterUrl
  const word = 'Dune'
  return (ctx, w, h, t) => {
    const u = w / 100 // one unit is a hundredth of the width
    const loop = t % 6
    ctx.fillStyle = '#0d0f13'
    ctx.fillRect(0, 0, w, h)
    ctx.textBaseline = 'middle'
    ctx.fillStyle = '#34d1bf'
    ctx.font = `600 ${5.5 * u}px Inter, system-ui, sans-serif`
    ctx.fillText('Searching on the TV', 7 * u, 26 * u)
    // The field, and the typing.
    ctx.fillStyle = 'rgba(255,255,255,0.08)'
    ctx.beginPath()
    ctx.roundRect(7 * u, 34 * u, 86 * u, 15 * u, 4 * u)
    ctx.fill()
    const typed = word.slice(0, Math.max(0, Math.min(word.length, Math.floor((loop - 0.6) / 0.3) + 1)))
    ctx.fillStyle = '#fff'
    ctx.font = `600 ${7 * u}px Inter, system-ui, sans-serif`
    ctx.fillText(typed, 12 * u, 41.5 * u)
    if (Math.floor(t * 2.5) % 2 === 0) {
      ctx.fillStyle = '#34d1bf'
      ctx.fillRect(12 * u + ctx.measureText(typed).width + 0.8 * u, 37 * u, 0.8 * u, 9 * u)
    }
    // The microphone.
    ctx.fillStyle = 'rgba(52,209,191,0.18)'
    ctx.beginPath()
    ctx.arc(50 * u, 66 * u, 13 * u, 0, Math.PI * 2)
    ctx.fill()
    ctx.fillStyle = '#34d1bf'
    ctx.beginPath()
    ctx.arc(50 * u, 66 * u, 9 * u, 0, Math.PI * 2)
    ctx.fill()
    // The film, once it's typed.
    const shown = Math.max(0, Math.min(1, (loop - 2.2) / 0.5)) * Math.max(0, Math.min(1, (5.8 - loop) / 0.4))
    if (shown > 0 && poster.complete && poster.naturalWidth) {
      ctx.globalAlpha = shown
      const pw = 86 * u
      const ph = pw * 1.5
      const y = 86 * u + (1 - shown) * 6 * u
      ctx.save()
      ctx.beginPath()
      ctx.roundRect(7 * u, y, pw, ph, 4 * u)
      ctx.clip()
      ctx.drawImage(poster, 7 * u, y, pw, ph)
      ctx.restore()
      ctx.globalAlpha = 1
    }
  }
}
