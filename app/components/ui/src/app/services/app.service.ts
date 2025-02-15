import { Injectable } from "@angular/core";
import { HttpClient } from "@angular/common/http";
import { Observable } from 'rxjs/internal/Observable';
import { APIRoutes } from "../constants";
import { environmentURLs } from "../environment";


export type HttpListResponseFailure = { status: string, message: string };


const endpoints = {
    getACEVersionData: `${environmentURLs.controller}${APIRoutes.ROOT}version`,
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
