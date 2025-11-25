export namespace main {
	
	export class Config {
	    window_width: number;
	    window_height: number;
	
	    static createFrom(source: any = {}) {
	        return new Config(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.window_width = source["window_width"];
	        this.window_height = source["window_height"];
	    }
	}
	export class FileDialogResponse {
	    file_path: string;
	    content: string;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new FileDialogResponse(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.file_path = source["file_path"];
	        this.content = source["content"];
	        this.error = source["error"];
	    }
	}

}

