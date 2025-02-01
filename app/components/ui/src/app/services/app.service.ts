import { Injectable } from "@angular/core";
import { HttpClient } from "@angular/common/http";
import { Observable } from 'rxjs/internal/Observable';
import { serviceURLs } from "../api.urls";


export type HttpListResponseFailure = { status: string, message: string };


const endpoints = {
    getACEVersionData: `${serviceURLs.controller}/version`,
};


@Injectable({
  providedIn: "root"
})
export class AppService {
  constructor(private http: HttpClient) { }

  getACEVersionData(): Observable<HttpListResponseFailure | any> {
      return this.http.get<HttpListResponseFailure | any>(endpoints.getACEVersionData);
  }
}
