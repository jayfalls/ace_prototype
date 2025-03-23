import { Injectable } from "@angular/core";
import { HttpClient } from "@angular/common/http";
import { Observable } from 'rxjs/internal/Observable';
import { APIRoutes } from "../constants";
import { environmentURLs } from "../environment";


export type HttpListResponseFailure = { status: string, message: string };

const endpoints = {
    getLLMModels: `${environmentURLs.controller}${APIRoutes.MODEL_PROVIDER}llm/models`,
    getLLMModelTypes: `${environmentURLs.controller}${APIRoutes.MODEL_PROVIDER}llm/model-types`
};


@Injectable({
  providedIn: "root"
})
export class ModelProviderService {
  constructor(private http: HttpClient) { }

  getLLMModelTypes(): Observable<HttpListResponseFailure | any> {
    return this.http.get<HttpListResponseFailure | any>(endpoints.getLLMModelTypes);
  }

  getLLMModels(): Observable<HttpListResponseFailure | any> {
    return this.http.get<HttpListResponseFailure | any>(endpoints.getLLMModels);
  }
}
