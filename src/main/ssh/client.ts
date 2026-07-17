import { Client } from 'ssh2'
import type { ConnectConfig } from 'ssh2'

export interface SshConfig {
  host: string
  port: number
  username: string
  password?: string
  privateKey?: string | Buffer
}

export interface ExecResult {
  stdout: string
  stderr: string
  code: number | null
}

/**
 * SSH 客户端：封装 ssh2 的连接/exec/sftp 操作。
 * 一个实例对应一个远程 host，懒连接，多次操作复用同一连接。
 */
export class SshClient {
  private client: Client
  private connected = false
  private connecting: Promise<void> | null = null
  private readonly config: ConnectConfig

  constructor(config: SshConfig) {
    this.config = {
      host: config.host,
      port: config.port,
      username: config.username,
      ...(config.password ? { password: config.password } : {}),
      ...(config.privateKey ? { privateKey: config.privateKey } : {}),
      readyTimeout: 10000
    }
    this.client = new Client()
  }

  async connect(): Promise<void> {
    if (this.connected) return
    if (this.connecting) return this.connecting
    this.connecting = new Promise<void>((resolve, reject) => {
      this.client.once('ready', () => {
        this.connected = true
        this.connecting = null
        resolve()
      })
      this.client.once('error', (err) => {
        this.connecting = null
        reject(err)
      })
      this.client.connect(this.config)
    })
    return this.connecting
  }

  async exec(command: string): Promise<ExecResult> {
    await this.connect()
    return new Promise((resolve, reject) => {
      this.client.exec(command, (err, stream) => {
        if (err) { reject(err); return }
        let stdout = ''
        let stderr = ''
        stream.on('data', (d: Buffer) => { stdout += d.toString() })
        stream.stderr.on('data', (d: Buffer) => { stderr += d.toString() })
        stream.on('close', (code: number | null) => {
          resolve({ stdout, stderr, code })
        })
      })
    })
  }

  async readFile(remotePath: string): Promise<string> {
    await this.connect()
    return new Promise((resolve, reject) => {
      this.client.sftp((err, sftp) => {
        if (err) { reject(err); return }
        const chunks: Buffer[] = []
        const stream = sftp.createReadStream(remotePath)
        stream.on('data', (d: Buffer) => chunks.push(d))
        stream.on('end', () => resolve(Buffer.concat(chunks).toString('utf8')))
        stream.on('error', reject)
      })
    })
  }

  async writeFile(remotePath: string, content: string): Promise<void> {
    await this.connect()
    return new Promise((resolve, reject) => {
      this.client.sftp((err, sftp) => {
        if (err) { reject(err); return }
        const stream = sftp.createWriteStream(remotePath)
        stream.on('finish', () => resolve())
        stream.on('error', reject)
        stream.end(content, 'utf8')
      })
    })
  }

  close(): void {
    if (this.connected) {
      this.client.end()
      this.connected = false
    }
  }
}
