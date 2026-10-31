export namespace core {
	
	export class Release {
	    component: string;
	    branch: string;
	    version: string;
	    url: string;
	
	    static createFrom(source: any = {}) {
	        return new Release(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.component = source["component"];
	        this.branch = source["branch"];
	        this.version = source["version"];
	        this.url = source["url"];
	    }
	}
	export class Catalog {
	    php: Release[];
	    nginx?: Release;
	    apache?: Release;
	    mariadb?: Release;
	    phpmyadmin?: Release;
	    composer?: Release;
	    errors: string[];
	
	    static createFrom(source: any = {}) {
	        return new Catalog(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.php = this.convertValues(source["php"], Release);
	        this.nginx = this.convertValues(source["nginx"], Release);
	        this.apache = this.convertValues(source["apache"], Release);
	        this.mariadb = this.convertValues(source["mariadb"], Release);
	        this.phpmyadmin = this.convertValues(source["phpmyadmin"], Release);
	        this.composer = this.convertValues(source["composer"], Release);
	        this.errors = source["errors"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ComponentState {
	    id: string;
	    name: string;
	    installed: boolean;
	    version: string;
	
	    static createFrom(source: any = {}) {
	        return new ComponentState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.installed = source["installed"];
	        this.version = source["version"];
	    }
	}
	export class SiteConfig {
	    php?: string;
	
	    static createFrom(source: any = {}) {
	        return new SiteConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.php = source["php"];
	    }
	}
	export class Config {
	    webServer: string;
	    httpPort: number;
	    ssl: boolean;
	    httpsPort: number;
	    dbPort: number;
	    tld: string;
	    defaultPhp: string;
	    phpWorkers: number;
	    autoStart: boolean;
	    closeToTray: boolean;
	    sites: Record<string, SiteConfig>;
	
	    static createFrom(source: any = {}) {
	        return new Config(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.webServer = source["webServer"];
	        this.httpPort = source["httpPort"];
	        this.ssl = source["ssl"];
	        this.httpsPort = source["httpsPort"];
	        this.dbPort = source["dbPort"];
	        this.tld = source["tld"];
	        this.defaultPhp = source["defaultPhp"];
	        this.phpWorkers = source["phpWorkers"];
	        this.autoStart = source["autoStart"];
	        this.closeToTray = source["closeToTray"];
	        this.sites = this.convertValues(source["sites"], SiteConfig, true);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class LogFile {
	    name: string;
	    size: number;
	
	    static createFrom(source: any = {}) {
	        return new LogFile(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.size = source["size"];
	    }
	}
	export class PHPExtension {
	    name: string;
	    enabled: boolean;
	
	    static createFrom(source: any = {}) {
	        return new PHPExtension(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.enabled = source["enabled"];
	    }
	}
	export class PHPState {
	    branch: string;
	    version: string;
	    running: boolean;
	    port: number;
	    default: boolean;
	    sites: number;
	
	    static createFrom(source: any = {}) {
	        return new PHPState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.branch = source["branch"];
	        this.version = source["version"];
	        this.running = source["running"];
	        this.port = source["port"];
	        this.default = source["default"];
	        this.sites = source["sites"];
	    }
	}
	
	export class SSLState {
	    enabled: boolean;
	    port: number;
	    caExists: boolean;
	    trusted: boolean;
	    thumbprint: string;
	    expires: string;
	
	    static createFrom(source: any = {}) {
	        return new SSLState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.port = source["port"];
	        this.caExists = source["caExists"];
	        this.trusted = source["trusted"];
	        this.thumbprint = source["thumbprint"];
	        this.expires = source["expires"];
	    }
	}
	export class ServiceState {
	    id: string;
	    name: string;
	    installed: boolean;
	    version: string;
	    running: boolean;
	    port: number;
	    error: string;
	
	    static createFrom(source: any = {}) {
	        return new ServiceState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.installed = source["installed"];
	        this.version = source["version"];
	        this.running = source["running"];
	        this.port = source["port"];
	        this.error = source["error"];
	    }
	}
	
	export class SiteState {
	    name: string;
	    domain: string;
	    url: string;
	    path: string;
	    docRoot: string;
	    php: string;
	    system: boolean;
	
	    static createFrom(source: any = {}) {
	        return new SiteState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.domain = source["domain"];
	        this.url = source["url"];
	        this.path = source["path"];
	        this.docRoot = source["docRoot"];
	        this.php = source["php"];
	        this.system = source["system"];
	    }
	}
	export class Task {
	    id: string;
	    label: string;
	    stage: string;
	    progress: number;
	    error: string;
	    done: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Task(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.label = source["label"];
	        this.stage = source["stage"];
	        this.progress = source["progress"];
	        this.error = source["error"];
	        this.done = source["done"];
	    }
	}
	export class State {
	    root: string;
	    config: Config;
	    services: ServiceState[];
	    php: PHPState[];
	    sites: SiteState[];
	    components: ComponentState[];
	    tasks: Task[];
	    vcRedist: boolean;
	    ssl: SSLState;
	    startUrl: string;
	
	    static createFrom(source: any = {}) {
	        return new State(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.root = source["root"];
	        this.config = this.convertValues(source["config"], Config);
	        this.services = this.convertValues(source["services"], ServiceState);
	        this.php = this.convertValues(source["php"], PHPState);
	        this.sites = this.convertValues(source["sites"], SiteState);
	        this.components = this.convertValues(source["components"], ComponentState);
	        this.tasks = this.convertValues(source["tasks"], Task);
	        this.vcRedist = source["vcRedist"];
	        this.ssl = this.convertValues(source["ssl"], SSLState);
	        this.startUrl = source["startUrl"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

